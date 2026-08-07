package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes is retained only as a source-compatible integration seam.
// Operator gateway-management routes live in RegisterOperatorRoutes.
func RegisterUserRoutes(
	_ *gin.RouterGroup,
	_ *handler.Handlers,
	_ middleware.JWTAuthMiddleware,
	_ middleware.AuditLogMiddleware,
	_ *service.SettingService,
) {
}
