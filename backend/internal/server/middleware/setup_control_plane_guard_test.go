package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupControlPlaneGuardAllowsLoopbackPeerAndHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupControlPlaneGuard())
	r.POST("/setup/install", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "http://localhost/setup/install", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestSetupControlPlaneGuardRejectsRemotePeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupControlPlaneGuard())
	r.POST("/setup/install", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "http://localhost/setup/install", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetupControlPlaneGuardRejectsLoopbackPeerWithUntrustedHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupControlPlaneGuard())
	r.POST("/setup/install", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "http://attacker.example/setup/install", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetupControlPlaneGuardAllowsExplicitPrivateHostAndCIDR(t *testing.T) {
	t.Setenv("SUB2API_PRIVATE_CONTROL_HOSTS", "100.97.17.1")
	t.Setenv("SUB2API_PRIVATE_CONTROL_CIDRS", "100.97.17.0/24")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SetupControlPlaneGuard())
	r.POST("/setup/install", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodPost, "http://100.97.17.1/setup/install", nil)
	req.RemoteAddr = "100.97.17.42:12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code)
}
