package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicControlPlaneGuardAllowsGatewayPathsOnPublicHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/v1/models", nil)
	req.Host = "sub2api.research.for-immortal.cn"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestPublicControlPlaneGuardBlocksControlPanelOnPublicHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/admin/dashboard", nil)
	req.Host = "sub2api.research.for-immortal.cn"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicControlPlaneGuardBlocksControlAPIsOnPublicHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.POST("/api/v1/auth/local-admin", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "https://sub2api.research.for-immortal.cn/api/v1/auth/local-admin", nil)
	req.Host = "sub2api.research.for-immortal.cn"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicControlPlaneGuardAllowsWireGuardControlPanel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/admin/dashboard", nil)
	req.Host = "100.97.17.1:8027"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestPublicControlPlaneGuardDisabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/admin/dashboard", nil)
	req.Host = "sub2api.research.for-immortal.cn"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}
