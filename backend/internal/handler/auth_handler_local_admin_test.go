//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLocalAdminBypassEnabled(t *testing.T) {
	t.Setenv("SUB2API_LOCAL_ADMIN_BYPASS", "")
	require.False(t, localAdminBypassEnabled())

	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SUB2API_LOCAL_ADMIN_BYPASS", value)
			require.True(t, localAdminBypassEnabled())
		})
	}

	t.Setenv("SUB2API_LOCAL_ADMIN_BYPASS", "false")
	require.False(t, localAdminBypassEnabled())
}

func TestIsLocalAdminBypassRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_LOCAL_ADMIN_BYPASS_CIDRS", "100.97.17.0/24")

	tests := []struct {
		name       string
		host       string
		remoteAddr string
		want       bool
	}{
		{name: "loopback host and loopback remote", host: "127.0.0.1:8027", remoteAddr: "127.0.0.1:55321", want: true},
		{name: "localhost host and docker bridge remote", host: "localhost:8027", remoteAddr: "172.18.0.1:55321", want: true},
		{name: "ipv6 loopback host", host: "[::1]:8027", remoteAddr: "[::1]:55321", want: true},
		{name: "wireguard host and docker bridge remote", host: "100.97.17.1:8027", remoteAddr: "172.18.0.1:55321", want: true},
		{name: "wireguard subnet host and wireguard peer remote", host: "100.97.17.1:8027", remoteAddr: "100.97.17.23:55321", want: true},
		{name: "public host rejected", host: "sub2api.research.for-immortal.cn", remoteAddr: "127.0.0.1:55321", want: false},
		{name: "public remote rejected", host: "127.0.0.1:8027", remoteAddr: "203.0.113.10:55321", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, "http://"+tt.host+"/api/v1/auth/local-admin", nil)
			req.Host = tt.host
			req.RemoteAddr = tt.remoteAddr
			c.Request = req

			require.Equal(t, tt.want, isLocalAdminBypassRequest(c))
		})
	}
}
