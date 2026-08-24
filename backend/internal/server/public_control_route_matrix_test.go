package server

import (
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
