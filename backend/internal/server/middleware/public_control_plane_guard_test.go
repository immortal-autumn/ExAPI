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

func TestPublicControlPlaneGuardRejectsGatewayLookalikes(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	for _, path := range []string{
		"/responses-fake",
		"/chat/completions-extra",
		"/embeddings-old",
		"/images/generations-fake",
		"/videos/request/extra",
	} {
		t.Run(path, func(t *testing.T) {
			require.False(t, isPublicGatewayPath(path))
		})
	}
}

func TestPublicControlPlaneGuardFailsClosedWhenPublicHostMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://unconfigured.example/admin/dashboard", nil)
	req.Host = "unconfigured.example"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicControlPlaneGuardFailsClosedForUnknownHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://attacker.invalid/admin/dashboard", nil)
	req.Host = "attacker.invalid"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestPublicControlPlaneGuardAllowsExplicitPrivateControlHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")
	t.Setenv("SUB2API_PRIVATE_CONTROL_HOSTS", "localhost,127.0.0.1,::1,100.97.17.1")
	t.Setenv("SUB2API_PRIVATE_CONTROL_CIDRS", "100.97.17.0/24")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/admin/dashboard", nil)
	req.Host = "100.97.17.1:8027"
	req.RemoteAddr = "100.97.17.25:43120"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestPublicControlPlaneGuardRejectsSpoofedPrivateHostFromRemotePeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	t.Setenv("SUB2API_PRIVATE_CONTROL_HOSTS", "localhost,127.0.0.1,100.97.17.1")
	t.Setenv("SUB2API_PRIVATE_CONTROL_CIDRS", "100.97.17.0/24")

	for _, host := range []string{"localhost", "127.0.0.1", "100.97.17.1"} {
		t.Run(host, func(t *testing.T) {
			r := gin.New()
			r.Use(PublicControlPlaneGuard())
			r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			req := httptest.NewRequest(http.MethodGet, "http://example.invalid/admin/dashboard", nil)
			req.Host = host
			req.RemoteAddr = "203.0.113.25:43120"
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestPublicControlPlaneGuardAlwaysUsesPrivateMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "false")
	t.Setenv("SUB2API_PUBLIC_HOST", "sub2api.research.for-immortal.cn")

	r := gin.New()
	r.Use(PublicControlPlaneGuard())
	r.GET("/admin/dashboard", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://sub2api.research.for-immortal.cn/admin/dashboard", nil)
	req.Host = "sub2api.research.for-immortal.cn"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The private-only revision intentionally ignores the removed mode switch;
	// a public host cannot expose the control plane even when the old variable
	// is set to false.
	require.Equal(t, http.StatusNotFound, w.Code)
}
