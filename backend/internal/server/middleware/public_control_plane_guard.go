package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/routecontract"
	"github.com/gin-gonic/gin"
)

// PublicControlPlaneGuard hides all non-gateway/control-plane paths from every
// host except the explicit SUB2API_PRIVATE_CONTROL_HOSTS allowlist when
// single-user private-control mode is enabled. This is defense in depth for
// nginx allowlists: public clients can still use AI gateway endpoints, while
// the SPA/admin/auth/user APIs remain available only on named private hosts.
func PublicControlPlaneGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.SingleUserPrivateControlPlaneEnabled() {
			c.Next()
			return
		}

		if isConfiguredPrivateControlHost(c.Request.Host) && isConfiguredPrivateControlPeer(c.Request.RemoteAddr) {
			c.Next()
			return
		}

		if isPublicGatewayPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		c.AbortWithStatus(http.StatusNotFound)
	}
}

// SetupControlPlaneGuard protects first-run setup before administrator
// authentication exists. Loopback requires both a loopback peer and Host;
// explicitly configured private hosts/CIDRs use the same fail-closed policy as
// the authenticated private control plane.
func SetupControlPlaneGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isLoopbackHost(c.Request.Host) && isLoopbackPeer(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		if isConfiguredPrivateControlHost(c.Request.Host) && isConfiguredPrivateControlPeer(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusNotFound)
	}
}

func isLoopbackHost(rawHost string) bool {
	host := normalizeHost(rawHost)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isLoopbackPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isConfiguredPrivateControlHost(rawHost string) bool {
	host := normalizeHost(rawHost)
	if host == "" {
		return false
	}
	for _, candidate := range strings.Split(os.Getenv("SUB2API_PRIVATE_CONTROL_HOSTS"), ",") {
		if normalizeHost(candidate) == host {
			return true
		}
	}
	return false
}

func isConfiguredPrivateControlPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return false
	}
	if peer.IsLoopback() {
		return true
	}
	for _, raw := range strings.Split(os.Getenv("SUB2API_PRIVATE_CONTROL_CIDRS"), ",") {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(peer) {
			return true
		}
	}
	return false
}

func normalizeHost(rawHost string) string {
	host := strings.TrimSpace(rawHost)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	return strings.ToLower(strings.TrimSpace(host))
}

func isPublicGatewayPath(path string) bool {
	return routecontract.IsPublicGatewayPath(path)
}
