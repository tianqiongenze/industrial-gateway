// Command gateway starts the industrial data-collection gateway: REST API +
// the ingestion scheduler that polls every collector on an interval.
// Infra: CockroachDB (read-write split across two nodes) + 2-level cache.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	httpadapter "industrial-gateway/internal/adapter/http"
	grpcadapter "industrial-gateway/internal/adapter/grpc"
	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/infra/crdbstore"
	"industrial-gateway/internal/infra/memoryrepo"
	"industrial-gateway/internal/infra/rediscache"
	"industrial-gateway/internal/usecase"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	logger := log.New(os.Stdout, "gateway ", log.LstdFlags|log.Lmsgprefix)

	// --- CRDB (CockroachDB) with read-write separation ---
	writeDSN := env("WRITE_DB", "host=10.167.2.175 port=26257 user=root dbname=industrial_gateway sslmode=disable")
	readDSN := env("READ_DB", "host=10.167.2.175 port=26267 user=root dbname=industrial_gateway sslmode=disable")
	store, err := crdbstore.NewStore(writeDSN, readDSN)
	if err != nil {
		logger.Fatalf("CRDB init failed: %v", err)
	}
	defer store.Close()
	logger.Printf("CRDB connected: WRITE=%s READ=%s", "10.167.2.175:26257", "10.167.2.175:26267")

	// --- two-level cache: L1 sync.Map (30s) + L2 Redis (300s) ---
	redisAddr := env("REDIS_ADDR", "redis.dify-plus.svc.cluster.local:6379")
	redisPass := env("REDIS_PASSWORD", "difyai123456")
	cache, err := rediscache.New(redisAddr, redisPass)
	if err != nil {
		logger.Fatalf("Redis init failed: %v", err)
	}

	// --- legacy in-memory hexagon (readings pipeline, kept for compat) ---
	repo := memoryrepo.New()
	t1 := domain.NewTransformer(
		map[string]string{"c": "Celsius", "f": "Celsius", "mm/s": "mm/s", "bar": "bar"},
		map[string]string{"rpm": "1"},
	)
	pipeline := &domain.Pipeline{Steps: []*domain.Transformer{t1}}
	uc := usecase.NewIngestService(pipeline, repo, cache /* collectors... */)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go runScheduler(ctx, uc, 5*time.Second, logger)

	// --- HTTP primary adapter ---
	engine := httpadapter.Register(uc, repo, cache)
	httpadapter.RegisterDeviceAPI(engine, store, cache)
	go func() {
		logger.Printf("REST API listening on :8080")
		if err := http.ListenAndServe(":8080", engine); err != nil {
			logger.Printf("http server stopped: %v", err)
		}
	}()

	// --- metrics endpoint ---
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	go func() {
		logger.Printf("metrics listening on :9090")
		_ = http.ListenAndServe(":9090", mux)
	}()

	// --- grpc adapter (logic-only skeleton; wired for tests) ---
	_ = grpcadapter.NewService(uc)

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Printf("shutting down...")
	stop()
	time.Sleep(500 * time.Millisecond)
}

func runScheduler(ctx context.Context, uc *usecase.IngestService, interval time.Duration, logger *log.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := uc.RunOnce(ctx)
			if err != nil {
				logger.Printf("ingest run error: %v", err)
			}
			if n > 0 {
				logger.Printf("ingested %d readings", n)
			}
		}
	}
}
