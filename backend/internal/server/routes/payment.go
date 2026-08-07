package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterPaymentRoutes is retained only as a source-compatible integration
// seam. Private ExAPI has no payment, checkout, webhook, or plan endpoints.
func RegisterPaymentRoutes(
	_ *gin.RouterGroup,
	_ *handler.PaymentHandler,
	_ *handler.PaymentWebhookHandler,
	_ *admin.PaymentHandler,
	_ middleware.JWTAuthMiddleware,
	_ middleware.AdminAuthMiddleware,
	_ middleware.AuditLogMiddleware,
	_ *service.SettingService,
	_ *middleware.PanelRateLimiter,
) {
}
