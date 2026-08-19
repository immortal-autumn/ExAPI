package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

const (
	ControlRequestHeader        = "X-ExAPI-Control-Request"
	ControlWebSocketSubprotocol = "exapi-control"
)

// ControlBoundary authenticates the network location of every request that
// reaches the private control listener. It intentionally uses RemoteAddr and
// never forwarded headers: the listener is expected to be bound directly to a
// WireGuard interface (or loopback for local maintenance).
func ControlBoundary(server config.ServerConfig) gin.HandlerFunc {
	allowedPeers := make(map[netip.Addr]struct{}, len(server.OperatorPeerIPs))
	for _, raw := range server.OperatorPeerIPs {
		if peer, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(raw), "[]")); err == nil {
			allowedPeers[peer.Unmap()] = struct{}{}
		}
	}
	allowedHosts := append([]string(nil), server.ControlHosts...)

	return func(c *gin.Context) {
		if c.Request == nil || !controlHostAllowed(c.Request.Host, allowedHosts) || !controlPeerAllowed(c.Request.RemoteAddr, allowedPeers) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		if strings.HasPrefix(c.Request.URL.Path, "/api/") && !controlRequestMarked(c.Request) {
			AbortWithError(c, http.StatusForbidden, "CONTROL_REQUEST_REQUIRED", "Private control request marker required")
			return
		}
		if !controlOriginAllowed(c.Request) || !controlFetchMetadataAllowed(c.Request) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

func controlHostAllowed(rawHost string, allowed []string) bool {
	authority := normalizeAuthority(rawHost)
	hostname := normalizeHostname(rawHost)
	if authority == "" || hostname == "" {
		return false
	}
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidateAuthority := normalizeAuthority(candidate); candidateAuthority == authority {
			return true
		}
		if !hasExplicitPort(candidate) && normalizeHostname(candidate) == hostname {
			return true
		}
	}
	return false
}

func controlPeerAllowed(remoteAddr string, allowed map[netip.Addr]struct{}) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	_, ok := allowed[peer.Unmap()]
	return ok
}

func controlOriginAllowed(request *http.Request) bool {
	raw := strings.TrimSpace(request.Header.Get("Origin"))
	unsafe := request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions
	if raw == "" {
		if isWebSocketRequest(request) {
			return true
		}
		if strings.HasPrefix(request.URL.Path, "/api/") {
			site := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")))
			return !unsafe && (site == "same-origin" || site == "none")
		}
		return !unsafe
	}
	origin, err := url.Parse(raw)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.User != nil || origin.Path != "" {
		return false
	}
	return normalizeAuthority(origin.Host) == normalizeAuthority(request.Host)
}

func controlFetchMetadataAllowed(request *http.Request) bool {
	site := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")))
	mode := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Mode")))
	unsafe := request.Method != http.MethodGet && request.Method != http.MethodHead && request.Method != http.MethodOptions
	// Fetch Metadata is a useful defense-in-depth signal, but browsers and
	// embedded clients are allowed to omit it (Chrome headless and some
	// WebViews do). The direct listener has already authenticated the exact
	// Host and WireGuard peer, and API requests still require the explicit
	// control marker. Keep the stricter same-origin requirement for mutations.
	if strings.HasPrefix(request.URL.Path, "/api/") && site == "" && !isWebSocketRequest(request) && unsafe {
		return false
	}
	if site != "" && site != "same-origin" && site != "none" {
		return false
	}
	if strings.HasPrefix(request.URL.Path, "/api/") && mode == "navigate" {
		return false
	}
	if unsafe && site != "same-origin" {
		return false
	}
	return true
}

func controlRequestMarked(request *http.Request) bool {
	if strings.TrimSpace(request.Header.Get(ControlRequestHeader)) == "1" {
		return true
	}
	if !isWebSocketRequest(request) {
		return false
	}
	for _, protocol := range strings.Split(request.Header.Get("Sec-WebSocket-Protocol"), ",") {
		if strings.TrimSpace(protocol) == ControlWebSocketSubprotocol {
			return true
		}
	}
	return false
}

func isWebSocketRequest(request *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(request.Header.Get("Upgrade")), "websocket") &&
		strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade")
}

func normalizeAuthority(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeHostname(raw string) string {
	host := strings.TrimSpace(raw)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
}

func hasExplicitPort(raw string) bool {
	_, _, err := net.SplitHostPort(strings.TrimSpace(raw))
	return err == nil
}
