package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

// PublicControlPlaneGuard hides all non-gateway/control-plane paths on the
// configured public host when single-user private-control mode is enabled.
// This is defense in depth for nginx allowlists: public clients can still use
// AI gateway endpoints, while the SPA/admin/auth/user APIs remain private to
// localhost/WireGuard direct access.
func PublicControlPlaneGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.SingleUserPrivateControlPlaneEnabled() {
			c.Next()
			return
		}

		if !isConfiguredPublicHost(c.Request.Host) {
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

func isConfiguredPublicHost(rawHost string) bool {
	publicHost := normalizeHost(os.Getenv("SUB2API_PUBLIC_HOST"))
	if publicHost == "" {
		return false
	}
	return normalizeHost(rawHost) == publicHost
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
	if path == "/health" {
		return true
	}

	prefixes := []string{
		"/v1/",
		"/v1beta/",
		"/backend-api/codex/",
		"/antigravity/",
		"/responses",
		"/chat/completions",
		"/embeddings",
		"/images/",
		"/videos/",
	}
	for _, prefix := range prefixes {
		if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
