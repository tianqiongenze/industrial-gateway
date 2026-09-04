package http

import (
	"github.com/gin-gonic/gin"

	"industrial-gateway/internal/infra/crdbstore"
	"industrial-gateway/internal/infra/rediscache"
)

// RegisterDeviceAPI mounts the device CRUD + cache + db info routes.
func RegisterDeviceAPI(r *gin.Engine, store *crdbstore.Store, cache *rediscache.Cache) {
	api := NewDeviceAPI(store, cache)
	g := r.Group("/api/v1")
	{
		g.POST("/devices", api.Create)
		g.GET("/devices", api.List)
		g.GET("/devices/:id", api.Get)
		g.PUT("/devices/:id", api.Update)
		g.DELETE("/devices/:id", api.Delete)
		g.GET("/cache/stats", api.CacheStats)
		g.GET("/db/info", api.DBInfo)
	}
}
