package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicRouterOmitsOperatorAndAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	// Route registration only takes method values; these empty handlers are
	// sufficient to inspect the public route graph without starting services.
	handlers := &handler.Handlers{
		Gateway:       &handler.GatewayHandler{},
		OpenAIGateway: &handler.OpenAIGatewayHandler{},
		AsyncImage:    &handler.AsyncImageHandler{},
		BatchImage:    &handler.BatchImageHandler{},
	}
	apiKeyAuth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) { c.Next() })

	SetupPublicRouter(router, handlers, apiKeyAuth, nil, nil, nil, nil, cfg, nil, nil)

	paths := make(map[string]struct{}, len(router.Routes()))
	for _, route := range router.Routes() {
		paths[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/operator/me",
		"GET /api/v1/admin/accounts/data",
		"GET /api/v1/admin/backups/:id/download-url",
		"GET /api/v1/operator/batch-images",
	} {
		_, registered := paths[route]
		require.Falsef(t, registered, "public router must omit private control route %s", route)
	}
}

func TestPublicRouterControlBoundaryAndRetiredSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_PRIVATE_CONTROL_HOSTS", "")
	t.Setenv("SUB2API_PRIVATE_CONTROL_CIDRS", "")

	router := gin.New()
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}
	t.Setenv("EXAPI_CONTROL_HOSTS", "")
	t.Setenv("EXAPI_OPERATOR_PEER_IPS", "")
	handlers := &handler.Handlers{
		Gateway:       &handler.GatewayHandler{},
		OpenAIGateway: &handler.OpenAIGatewayHandler{},
		AsyncImage:    &handler.AsyncImageHandler{},
		BatchImage:    &handler.BatchImageHandler{},
	}
	apiKeyAuth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	SetupPublicRouter(router, handlers, apiKeyAuth, nil, nil, nil, nil, cfg, nil, nil)
	// This route models a future registration mistake. The public listener must
	// remain safe even if a control endpoint is added to the wrong engine.
	router.Any("/admin/control-boundary-sentinel", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "health", method: http.MethodGet, path: "/health", status: http.StatusOK},
		{name: "ready", method: http.MethodGet, path: "/ready", status: http.StatusServiceUnavailable},
		{name: "gateway", method: http.MethodGet, path: "/v1/models", status: http.StatusUnauthorized},
		{name: "retired-get", method: http.MethodGet, path: "/api/v1/user/profile", status: http.StatusGone},
		{name: "retired-post", method: http.MethodPost, path: "/api/v1/user/profile", status: http.StatusGone},
		{name: "retired-delete", method: http.MethodDelete, path: "/api/v1/user/profile", status: http.StatusGone},
		{name: "control-panel", method: http.MethodGet, path: "/admin/control-boundary-sentinel", status: http.StatusNotFound},
		{name: "control-api", method: http.MethodGet, path: "/api/v1/admin/control-boundary-sentinel", status: http.StatusNotFound},
		{name: "setup", method: http.MethodGet, path: "/setup", status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "https://public.example"+tt.path, nil)
			req.Host = "public.example"
			req.RemoteAddr = "203.0.113.25:43120"
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			require.Equal(t, tt.status, recorder.Code)
		})
	}
}
