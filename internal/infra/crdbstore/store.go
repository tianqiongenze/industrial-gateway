// Package crdbstore implements domain ports backed by CockroachDB (CRDB).
// CRDB speaks the PostgreSQL wire protocol, so lib/pq is used.
// Read/write separation: every mutation goes through writeDB (*:26257),
// every read goes through readDB (:26267, a CRDB read node).
package crdbstore

import (
	"database/sql"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"

	"industrial-gateway/internal/domain"
)

const driverName = "postgres"

// Store is a CRDB-backed store with read/write splitting.
type Store struct {
	writeDB *sql.DB
	readDB  *sql.DB
}

// NewStore opens two pools: write (master node A) and read (master node B).
func NewStore(writeDSN, readDSN string) (*Store, error) {
	w, err := openPool(writeDSN, 20)
	if err != nil {
		return nil, fmt.Errorf("write pool: %w", err)
	}
	r, err := openPool(readDSN, 30)
	if err != nil {
		w.Close()
		return nil, fmt.Errorf("read pool: %w", err)
	}
	return &Store{writeDB: w, readDB: r}, nil
}

func openPool(dsn string, maxOpen int) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen / 2)
	db.SetConnMaxLifetime(30 * time.Minute)
	// CRDB-friendly: avoid lib/pq prepared-statement default issues
	db.Exec("SET statement_timeout='30s'")
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// PingRead verifies read-node connectivity.
func (s *Store) PingRead() error { return s.readDB.Ping() }

// Close closes both pools.
func (s *Store) Close() {
	s.writeDB.Close()
	s.readDB.Close()
}

var ErrNotFound = domain.ErrNotFound

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}

// ---------- Device CRUD (write -> writeDB, read -> readDB) ----------

const deviceCols = `id, name, type, location, status, created_at, updated_at`

func scanDevice(row interface{ Scan(...interface{}) error }) (*domain.Device, error) {
	var d domain.Device
	var id, loc string
	var status string
	var created, updated time.Time
	if err := row.Scan(&id, &d.Name, &d.Type, &loc, &status, &created, &updated); err != nil {
		return nil, err
	}
	d.ID, d.Location, d.Status, d.CreatedAt, d.UpdatedAt = id, loc, status, created, updated
	return &d, nil
}

// CreateDevice INSERTs via the WRITE pool and returns the CRDB-generated UUID.
func (s *Store) CreateDevice(d *domain.Device) error {
	d.Status = orDefault(d.Status, "active")
	var id string
	err := s.writeDB.QueryRow(
		`INSERT INTO devices (name, type, location, status) VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		d.Name, d.Type, d.Location, d.Status,
	).Scan(&id, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert device: %w", err)
	}
	d.ID = id
	return nil
}

// GetDevice SELECTs via the READ pool.
func (s *Store) GetDevice(id string) (*domain.Device, error) {
	row := s.readDB.QueryRow(`SELECT `+deviceCols+` FROM devices WHERE id = $1`, id)
	d, err := scanDevice(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return d, err
}

// ListDevices SELECTs via the READ pool.
func (s *Store) ListDevices() ([]domain.Device, error) {
	rows, err := s.readDB.Query(`SELECT ` + deviceCols + ` FROM devices ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// UpdateDevice UPDATEs via the WRITE pool.
func (s *Store) UpdateDevice(id string, d *domain.Device) error {
	res, err := s.writeDB.Exec(
		`UPDATE devices SET name=$2, type=$3, location=$4, status=$5, updated_at=now() WHERE id=$1`,
		id, d.Name, d.Type, d.Location, orDefault(d.Status, "active"),
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteDevice DELETEs via the WRITE pool.
func (s *Store) DeleteDevice(id string) error {
	res, err := s.writeDB.Exec(`DELETE FROM devices WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountDevices SELECTs via the READ pool.
func (s *Store) CountDevices() (int64, error) {
	var n int64
	err := s.readDB.QueryRow(`SELECT count(*) FROM devices`).Scan(&n)
	return n, err
}

// ---------- Readings persistence (from ingest) ----------

// SaveReading writes a data point through the WRITE pool.
func (s *Store) SaveReading(r domain.Reading, deviceID string) error {
	_, err := s.writeDB.Exec(
		`INSERT INTO data_points (device_id, metric, value, unit, ts) VALUES ($1,$2,$3,$4,now())`,
		deviceID, r.Metric, r.Value, r.Unit,
	)
	return err
}

// LatestReadings reads via the READ pool.
func (s *Store) LatestReadings(deviceID string, n int) ([]map[string]interface{}, error) {
	rows, err := s.readDB.Query(
		`SELECT metric, value, unit, ts FROM data_points WHERE device_id=$1 ORDER BY ts DESC LIMIT $2`,
		deviceID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var m string
		var v float64
		var u string
		var ts time.Time
		if err := rows.Scan(&m, &v, &u, &ts); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{"metric": m, "value": v, "unit": u, "ts": ts})
	}
	return out, rows.Err()
}

// ---------- Cache metrics (read from READ pool) ----------

var cacheHits, cacheMisses, l2Hits int64

// RecordCacheHit records an L1 hit.
func RecordCacheHit() { atomic.AddInt64(&cacheHits, 1) }

// RecordCacheMiss records an L1 miss.
func RecordCacheMiss() { atomic.AddInt64(&cacheMisses, 1) }

// RecordL2Hit records an L2 (Redis) hit.
func RecordL2Hit() { atomic.AddInt64(&l2Hits, 1) }

// CacheStats returns current counters.
func CacheStats() (hits, misses, l2 int64) {
	return atomic.LoadInt64(&cacheHits), atomic.LoadInt64(&cacheMisses), atomic.LoadInt64(&l2Hits)
}

// WriteNode reports which node handles writes (for diagnostics).
func (s *Store) WriteNode() string {
	var v string
	s.writeDB.QueryRow(`SELECT gateway_configs.value FROM gateway_configs WHERE key='write_node' LIMIT 1`).Scan(&v)
	if v == "" {
		return "10.167.2.175:26257 (node A)"
	}
	return v
}

func init() { log.SetFlags(log.LstdFlags) }

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
