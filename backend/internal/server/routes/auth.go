package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterAuthRoutes is retained only as a source-compatible integration seam.
// Private ExAPI never registers browser/customer credential routes.
func RegisterAuthRoutes(
	_ *gin.RouterGroup,
	_ *handler.Handlers,
	_ servermiddleware.JWTAuthMiddleware,
	_ servermiddleware.AuditLogMiddleware,
	_ *redis.Client,
	_ *service.SettingService,
) {
}
