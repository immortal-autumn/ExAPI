package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestPrivateUserRouteMatrix(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	h := &handler.Handlers{
		User:             &handler.UserHandler{},
		APIKey:           &handler.APIKeyHandler{},
		Usage:            &handler.UsageHandler{},
		Totp:             &handler.TotpHandler{},
		ChannelMonitor:   &handler.ChannelMonitorUserHandler{},
		AvailableChannel: &handler.AvailableChannelHandler{},
	}
	auth := servermiddleware.OperatorAuthMiddleware(func(c *gin.Context) { c.Next() })
	audit := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	RegisterOperatorRoutes(v1, h, auth, audit, nil)

	paths := make(map[string]struct{})
	for _, route := range router.Routes() {
		paths[route.Path] = struct{}{}
	}

	for _, path := range []string{
		"/api/v1/keys",
		"/api/v1/groups/available",
		"/api/v1/groups/rates",
		"/api/v1/usage",
		"/api/v1/channel-monitors",
	} {
		if _, ok := paths[path]; !ok {
			t.Errorf("required private route %s is not registered", path)
		}
	}

	for _, path := range []string{
		"/api/v1/user/aff",
		"/api/v1/user/notify-email/send-code",
		"/api/v1/channels/available",
		"/api/v1/announcements",
		"/api/v1/redeem",
		"/api/v1/subscriptions",
	} {
		if _, ok := paths[path]; ok {
			t.Errorf("SaaS route %s is registered in private mode", path)
		}
	}
}

func TestPrivateAdminRouteMatrix(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	h := &handler.Handlers{Admin: &handler.AdminHandlers{
		Dashboard:              &adminhandler.DashboardHandler{},
		Group:                  &adminhandler.GroupHandler{},
		Account:                &adminhandler.AccountHandler{},
		Compliance:             &adminhandler.ComplianceHandler{},
		OAuth:                  &adminhandler.OAuthHandler{},
		OpenAIOAuth:            &adminhandler.OpenAIOAuthHandler{},
		GeminiOAuth:            &adminhandler.GeminiOAuthHandler{},
		AntigravityOAuth:       &adminhandler.AntigravityOAuthHandler{},
		GrokOAuth:              &adminhandler.GrokOAuthHandler{},
		Proxy:                  &adminhandler.ProxyHandler{},
		Setting:                &adminhandler.SettingHandler{},
		DataManagement:         &adminhandler.DataManagementHandler{},
		Backup:                 &adminhandler.BackupHandler{},
		Ops:                    &adminhandler.OpsHandler{},
		System:                 &adminhandler.SystemHandler{},
		Usage:                  &adminhandler.UsageHandler{},
		ErrorPassthrough:       &adminhandler.ErrorPassthroughHandler{},
		TLSFingerprintProfile:  &adminhandler.TLSFingerprintProfileHandler{},
		APIKey:                 &adminhandler.AdminAPIKeyHandler{},
		ScheduledTest:          &adminhandler.ScheduledTestHandler{},
		Channel:                &adminhandler.ChannelHandler{},
		ChannelMonitor:         &adminhandler.ChannelMonitorHandler{},
		ChannelMonitorTemplate: &adminhandler.ChannelMonitorRequestTemplateHandler{},
		ContentModeration:      &adminhandler.ContentModerationHandler{},
	}}
	auth := servermiddleware.OperatorAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterPrivateAdminRoutes(v1, h, auth, nil, nil, nil)

	paths := make(map[string]struct{})
	for _, route := range router.Routes() {
		paths[route.Path] = struct{}{}
	}

	for _, path := range []string{
		"/api/v1/admin/accounts",
		"/api/v1/admin/groups",
		"/api/v1/admin/settings",
		"/api/v1/admin/backups",
		"/api/v1/admin/ops/concurrency",
		"/api/v1/admin/usage",
		"/api/v1/admin/api-keys/:id",
	} {
		if _, ok := paths[path]; !ok {
			t.Errorf("required private admin route %s is not registered", path)
		}
	}

	for _, path := range []string{
		"/api/v1/admin/users",
		"/api/v1/admin/announcements",
		"/api/v1/admin/redeem-codes",
		"/api/v1/admin/promo-codes",
		"/api/v1/admin/subscriptions",
		"/api/v1/admin/user-attributes",
		"/api/v1/admin/affiliates/users",
		"/api/v1/admin/backups/:id/restore",
	} {
		if _, ok := paths[path]; ok {
			t.Errorf("SaaS admin route %s is registered in private mode", path)
		}
	}
}

func TestPrivateAuthRouteMatrix(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	h := &handler.Handlers{
		Auth:    &handler.AuthHandler{},
		Setting: &handler.SettingHandler{},
	}
	auth := servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAuthRoutes(v1, h, auth, nil, nil, nil)

	paths := make(map[string]struct{})
	for _, route := range router.Routes() {
		paths[route.Path] = struct{}{}
	}

	for _, path := range []string{
		"/api/v1/auth/login",
		"/api/v1/auth/local-admin",
		"/api/v1/auth/login/2fa",
		"/api/v1/auth/refresh",
		"/api/v1/auth/logout",
		"/api/v1/auth/me",
		"/api/v1/auth/revoke-all-sessions",
		"/api/v1/auth/register",
		"/api/v1/auth/send-verify-code",
		"/api/v1/auth/validate-promo-code",
		"/api/v1/auth/forgot-password",
		"/api/v1/auth/oauth/github/start",
		"/api/v1/auth/oauth/wechat/payment/start",
		"/api/v1/settings/email-unsubscribe",
		"/api/v1/auth/oauth/bind-token",
	} {
		if _, ok := paths[path]; ok {
			t.Errorf("SaaS auth route %s is registered in private mode", path)
		}
	}
}
