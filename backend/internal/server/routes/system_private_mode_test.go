package routes

import (
	"testing"

	rootHandler "github.com/Wei-Shaw/sub2api/internal/handler"
	adminHandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterSystemRoutesOmitsUpdaterEndpointsInPrivateMode(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	group := r.Group("/api/v1/admin")
	h := &rootHandler.Handlers{Admin: &rootHandler.AdminHandlers{System: &adminHandler.SystemHandler{}}}

	registerSystemRoutes(group, h)

	paths := map[string]bool{}
	for _, route := range r.Routes() {
		paths[route.Path] = true
	}
	require.True(t, paths["/api/v1/admin/system/version"])
	require.False(t, paths["/api/v1/admin/system/check-updates"])
	require.False(t, paths["/api/v1/admin/system/rollback-versions"])
	require.False(t, paths["/api/v1/admin/system/update"])
	require.False(t, paths["/api/v1/admin/system/rollback"])
	require.False(t, paths["/api/v1/admin/system/restart"])
}
