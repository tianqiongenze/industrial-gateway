// Package httpadapter is the primary (driving) adapter exposing a Gin REST API.
package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/usecase"
)

// IngestRequest is the JSON body for manual ingestion.
type IngestRequest struct {
	DeviceID string  `json:"device_id" binding:"required"`
	Metric   string  `json:"metric" binding:"required"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Source   string  `json:"source"`
}

// IngestResponse echoes the transformed reading + status.
type IngestResponse struct {
	domain.Reading
	Status string `json:"status"`
}

// Register builds the Gin engine from the use case + repository + cache.
func Register(uc *usecase.IngestService, repo domain.RepositoryPort,
	cache domain.CachePort) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	{
		api.GET("/health", healthHandler(repo, cache))
		api.POST("/ingest", ingestHandler(uc, repo))
		api.GET("/readings/:device", latestHandler(repo))
		api.GET("/status/:device", statusHandler(uc, cache))
	}
	return r
}

func healthHandler(repo domain.RepositoryPort, cache domain.CachePort) gin.HandlerFunc {
	return func(c *gin.Context) {
		count, _ := repo.Count("")
		c.JSON(http.StatusOK, gin.H{
			"status": "ok", "readings": count,
			"cache": cacheTypeName(cache), "time": time.Now().UTC(),
		})
	}
}

func ingestHandler(uc *usecase.IngestService, repo domain.RepositoryPort) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req IngestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		reading := domain.Reading{
			DeviceID: req.DeviceID, Metric: req.Metric, Value: req.Value,
			Unit: req.Unit, Timestamp: time.Now().UTC(),
			Source: orDefault(req.Source, "rest"), Quality: "good",
		}
		if err := reading.Validate(); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		if err := repo.Save(reading); err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		status := domain.Classify(reading)
		c.JSON(http.StatusCreated, IngestResponse{Reading: reading, Status: status})
	}
}

func latestHandler(repo domain.RepositoryPort) gin.HandlerFunc {
	return func(c *gin.Context) {
		device := c.Param("device")
		readings, _ := repo.Latest(device, "", 10)
		c.JSON(http.StatusOK, gin.H{"device_id": device, "readings": readings})
	}
}

func statusHandler(uc *usecase.IngestService, cache domain.CachePort) gin.HandlerFunc {
	return func(c *gin.Context) {
		device := c.Param("device")
		status := uc.StatusFor(device)
		c.JSON(http.StatusOK, gin.H{"device_id": device, "status": status})
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// cacheTypeName returns a friendly cache implementation name for /health.
func cacheTypeName(c domain.CachePort) string {
	if c == nil {
		return "none"
	}
	type namer interface{ Name() string }
	if n, ok := c.(namer); ok {
		return n.Name()
	}
	return "memorycache"
}
