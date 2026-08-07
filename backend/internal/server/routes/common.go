package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const readinessTimeout = 2 * time.Second

type ReadinessProbe func(context.Context) error

// RegisterCommonRoutes exposes only process/dependency probes shared by the
// public and control listeners. Setup and browser telemetry endpoints are not
// part of the private-only runtime contract.
func RegisterCommonRoutes(r *gin.Engine, readinessProbe ReadinessProbe) {
	// 存活检查只反映进程是否能处理 HTTP 请求。
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 就绪检查验证关键依赖，但不向未认证调用者泄露失败详情。
	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
		defer cancel()
		if readinessProbe == nil || readinessProbe(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
