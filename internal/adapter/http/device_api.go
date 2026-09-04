package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/infra/crdbstore"
	"industrial-gateway/internal/infra/rediscache"
)

// DeviceAPI wires device CRUD: CRDB (read-write split) + two-level cache.
type DeviceAPI struct {
	store *crdbstore.Store
	cache *rediscache.Cache
}

// NewDeviceAPI builds the handler set.
func NewDeviceAPI(store *crdbstore.Store, cache *rediscache.Cache) *DeviceAPI {
	return &DeviceAPI{store: store, cache: cache}
}

func (a *DeviceAPI) deviceKey(id string) string { return "device:" + id }

// Create handles POST /api/v1/devices (WRITE path).
func (a *DeviceAPI) Create(c *gin.Context) {
	var d domain.Device
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := d.Validate(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := a.store.CreateDevice(&d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// WRITE pattern: DB write succeeded -> populate cache
	_ = a.cache.SetJSON(a.deviceKey(d.ID), d)
	c.JSON(http.StatusCreated, d)
}

// Get handles GET /api/v1/devices/:id (READ path with 2-level cache).
func (a *DeviceAPI) Get(c *gin.Context) {
	id := c.Param("id")
	key := a.deviceKey(id)
	var d domain.Device
	found, _ := a.cache.GetJSON(key, &d)
	if found {
		c.Header("X-Cache", "HIT")
		c.JSON(http.StatusOK, d)
		return
	}
	d2, err := a.store.GetDevice(id)
	if err == domain.ErrNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.cache.SetJSON(key, *d2)
	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, d2)
}

// List handles GET /api/v1/devices (READ path with 2-level cache).
func (a *DeviceAPI) List(c *gin.Context) {
	key := "devices:list"
	var list []domain.Device
	found, _ := a.cache.GetJSON(key, &list)
	if found && list != nil {
		c.Header("X-Cache", "HIT")
		c.JSON(http.StatusOK, gin.H{"count": len(list), "devices": list})
		return
	}
	list, err := a.store.ListDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.cache.SetJSON(key, list)
	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, gin.H{"count": len(list), "devices": list})
}

// Update handles PUT /api/v1/devices/:id (WRITE path).
func (a *DeviceAPI) Update(c *gin.Context) {
	id := c.Param("id")
	var d domain.Device
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := d.Validate(); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	if err := a.store.UpdateDevice(id, &d); err != nil {
		if err == domain.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.cache.Delete(a.deviceKey(id)) // invalidate L2 then L1
	a.cache.Delete("devices:list")
	d.ID = id
	c.JSON(http.StatusOK, d)
}

// Delete handles DELETE /api/v1/devices/:id (WRITE path).
func (a *DeviceAPI) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := a.store.DeleteDevice(id); err != nil {
		if err == domain.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.cache.Delete(a.deviceKey(id))
	a.cache.Delete("devices:list")
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// CacheStats handles GET /api/v1/cache/stats.
func (a *DeviceAPI) CacheStats(c *gin.Context) {
	s := rediscache.Stats()
	var rate float64
	if total := s.L1Hits + s.L1Misses; total > 0 {
		rate = float64(s.L1Hits) / float64(total) * 100
	}
	c.JSON(http.StatusOK, gin.H{
		"l1_hits": s.L1Hits, "l1_misses": s.L1Misses,
		"l2_hits": s.L2Hits, "l2_misses": s.L2Misses, "l2_errors": s.L2Errs,
		"sets": s.Sets, "deletes": s.Deletes,
		"l1_hit_rate_pct": rate,
	})
}

// DBInfo handles GET /api/v1/db/info - documents the read/write split.
func (a *DeviceAPI) DBInfo(c *gin.Context) {
	readOK := a.store.PingRead() == nil
	c.JSON(http.StatusOK, gin.H{
		"write_dsn":           "10.167.2.175:26257/industrial_gateway (node A, writes)",
		"read_dsn":            "10.167.2.175:26267/industrial_gateway (node B, reads)",
		"read_node_reachable": readOK,
		"driver":              "github.com/lib/pq (CRDB PostgreSQL wire protocol)",
		"note":                "INSERT/UPDATE/DELETE -> write pool; SELECT -> read pool",
	})
}
