// Package outbound provides reusable destination policy for outbound HTTP clients.
// It resolves and validates every direct socket immediately before dialing, then
// pins the connection to the validated IP address to prevent DNS rebinding TOCTOU.
package outbound

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Resolver is the subset of net.Resolver used by Policy.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// Dialer is the subset of net.Dialer used by Policy.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// ResolutionMode identifies who resolves the final destination. Direct
// resolution is protected by socket-level IP pinning. A configured proxy is an
// explicit trust boundary and retains responsibility for destination DNS and
// egress policy; replacing its dialer would silently bypass the proxy.
type ResolutionMode uint8

const (
	DirectResolution ResolutionMode = iota
	TrustedProxyResolution
)

// Policy controls direct outbound destinations. Private and special-use
// addresses are denied unless AllowPrivate is explicitly enabled.
type Policy struct {
	Resolver     Resolver
	Dialer       Dialer
	AllowPrivate bool
}

// PublicPolicy returns the fail-closed default policy for public destinations.
func PublicPolicy() Policy {
	return Policy{}
}

func (p Policy) resolver() Resolver {
	if p.Resolver != nil {
		return p.Resolver
	}
	return net.DefaultResolver
}

func (p Policy) dialer() Dialer {
	if p.Dialer != nil {
		return p.Dialer
	}
	return &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
}

// ConfigureTransport installs the safe direct dialer. In trusted-proxy mode it
// deliberately preserves the proxy dialer because destination resolution occurs
// beyond this process and must be enforced by the explicitly configured proxy.
func (p Policy) ConfigureTransport(transport *http.Transport, mode ResolutionMode) error {
	if transport == nil {
		return errors.New("outbound transport is nil")
	}
	switch mode {
	case DirectResolution:
		transport.DialContext = p.DialContext
	case TrustedProxyResolution:
		// Preserve the configured HTTP/SOCKS proxy transport exactly.
	default:
		return fmt.Errorf("unsupported outbound resolution mode: %d", mode)
	}
	return nil
}

// DialContext resolves every hostname for this new socket, rejects the entire
// answer set if any address violates policy, and dials only validated literal
// IPs. DNS is never consulted again by the underlying dialer.
func (p Policy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound address: %w", err)
	}
	if err := validateHostLiteral(host, p.AllowPrivate); err != nil {
		return nil, err
	}

	if ip := net.ParseIP(host); ip != nil {
		return p.dialer().DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	addrs, err := p.resolver().LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("outbound host %q resolved to no addresses", host)
	}
	for _, addr := range addrs {
		if err := validateIP(addr.IP, p.AllowPrivate); err != nil {
			return nil, fmt.Errorf("outbound host %q resolved to a disallowed address: %w", host, err)
		}
	}

	var lastErr error
	for _, addr := range addrs {
		conn, dialErr := p.dialer().DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("no usable outbound address")
	}
	return nil, lastErr
}

// ValidateURL validates URL structure and literal destination policy. Hostname
// answers are intentionally validated at direct dial time, not here.
func (p Policy) ValidateURL(raw string, allowHTTP bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("invalid outbound URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" && !(allowHTTP && scheme == "http") {
		return nil, errors.New("outbound URL must use https")
	}
	if parsed.User != nil {
		return nil, errors.New("outbound URL credentials are not allowed")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("outbound URL host is empty")
	}
	if port := parsed.Port(); port != "" {
		value, convErr := strconv.Atoi(port)
		if convErr != nil || value < 1 || value > 65535 {
			return nil, errors.New("outbound URL port is invalid")
		}
	}
	if err := validateHostLiteral(parsed.Hostname(), p.AllowPrivate); err != nil {
		return nil, err
	}
	return parsed, nil
}

// RedirectChecker revalidates every redirect URL. The eventual direct socket is
// independently resolved, validated, and pinned by DialContext.
func (p Policy) RedirectChecker(allowHTTP bool, limit int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if limit > 0 && len(via) >= limit {
			return fmt.Errorf("stopped after %d redirects", limit)
		}
		if req == nil || req.URL == nil {
			return errors.New("invalid outbound redirect")
		}
		_, err := p.ValidateURL(req.URL.String(), allowHTTP)
		return err
	}
}

func validateHostLiteral(host string, allowPrivate bool) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return errors.New("outbound host is empty")
	}
	if !allowPrivate {
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || isMetadataHostname(host) {
			return fmt.Errorf("outbound host %q is disallowed", host)
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		return validateIP(ip, allowPrivate)
	}
	return nil
}

func validateIP(ip net.IP, allowPrivate bool) error {
	if ip == nil {
		return errors.New("invalid outbound IP")
	}
	if allowPrivate {
		return nil
	}
	for _, network := range blockedNetworks {
		if network.Contains(ip) {
			return fmt.Errorf("outbound IP %s is disallowed", ip.String())
		}
	}
	if !ip.IsGlobalUnicast() {
		return fmt.Errorf("outbound IP %s is disallowed", ip.String())
	}
	return nil
}

func isMetadataHostname(host string) bool {
	switch host {
	case "metadata", "metadata.google.internal", "metadata.goog", "instance-data", "instance-data.ec2.internal":
		return true
	default:
		return false
	}
}

var blockedNetworks = mustParseNetworks([]string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"168.63.129.16/32",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"2001::/32",
	"2001:db8::/32",
	"2002::/16",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
})

func mustParseNetworks(values []string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic("invalid outbound policy CIDR: " + value)
		}
		result = append(result, network)
	}
	return result
}
