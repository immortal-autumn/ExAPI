package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// RegisterOperatorRoutes exposes the former per-user gateway-management APIs
// to the singleton private operator. Authentication is inherited from the
// direct WireGuard peer boundary; no browser or API token is accepted.
func RegisterOperatorRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	operatorAuth middleware.OperatorAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
	panelRateLimiter *middleware.PanelRateLimiter,
) {
	operator := v1.Group("")
	operator.Use(gin.HandlerFunc(operatorAuth))
	operator.Use(panelRateLimiter.Global())
	operator.Use(gin.HandlerFunc(auditLog))

	operator.GET("/operator/me", func(c *gin.Context) {
		current, ok := middleware.GetOperatorFromContext(c)
		if !ok {
			middleware.AbortWithError(c, http.StatusServiceUnavailable, "OPERATOR_UNAVAILABLE", "Private operator unavailable")
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id":          current.ID,
			"username":    current.Username,
			"email":       current.Email,
			"role":        "admin",
			"status":      current.Status,
			"concurrency": current.Concurrency,
		})
	})
	// This legacy path is retained for frontend configuration bootstrap, but it
	// is private operator data now and inherits the peer/operator boundary.
	operator.GET("/settings/public", h.Setting.GetPublicSettings)

	keys := operator.Group("/keys")
	{
		keys.GET("", h.APIKey.List)
		keys.GET("/:id", h.APIKey.GetByID)
		keys.POST("", h.APIKey.Create)
		keys.PUT("/:id", h.APIKey.Update)
		keys.DELETE("/:id", h.APIKey.Delete)
	}

	groups := operator.Group("/groups")
	{
		groups.GET("/available", h.APIKey.GetAvailableGroups)
		groups.GET("/rates", h.APIKey.GetUserGroupRates)
	}

	usage := operator.Group("/usage")
	{
		usage.GET("", h.Usage.List)
		usage.GET("/errors", h.Usage.ListErrors)
		usage.GET("/errors/:id", h.Usage.GetErrorDetail)
		usage.GET("/stats", h.Usage.Stats)
		usage.GET("/dashboard/stats", h.Usage.DashboardStats)
		usage.GET("/dashboard/trend", h.Usage.DashboardTrend)
		usage.GET("/dashboard/models", h.Usage.DashboardModels)
		usage.GET("/dashboard/snapshot-v2", h.Usage.DashboardSnapshotV2)
		usage.POST("/dashboard/api-keys-usage", h.Usage.DashboardAPIKeysUsage)
		usage.GET("/:id", h.Usage.GetByID)
	}

	monitors := operator.Group("/channel-monitors")
	{
		monitors.GET("", h.ChannelMonitor.List)
		monitors.GET("/:id/status", h.ChannelMonitor.GetStatus)
	}
}
