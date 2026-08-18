package repository

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var backupBlockedHostnames = map[string]struct{}{
	"localhost":                  {},
	"localhost.localdomain":      {},
	"metadata":                   {},
	"metadata.google.internal":   {},
	"metadata.goog":              {},
	"instance-data":              {},
	"instance-data.ec2.internal": {},
}

var backupBlockedCIDRs = mustParseBackupCIDRs([]string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
})

func validateBackupEndpoint(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid backup S3 endpoint")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", errors.New("backup S3 endpoint must use https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("backup S3 endpoint must not contain credentials, query, or fragment")
	}
	if err := validateBackupHostLiteral(parsed.Hostname()); err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validateBackupHostLiteral(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return errors.New("backup S3 endpoint host is empty")
	}
	if _, blocked := backupBlockedHostnames[host]; blocked || strings.HasSuffix(host, ".localhost") {
		return errors.New("backup S3 endpoint host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedBackupIP(ip) {
		return errors.New("backup S3 endpoint IP is not allowed")
	}
	return nil
}

func newBackupHTTPClient() *http.Client {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		defaultTransport = &http.Transport{}
	}
	transport := defaultTransport.Clone()
	// Backup destinations must always pass through the SSRF-safe dialer. An
	// environment proxy would move the real network hop outside this policy.
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if err := validateBackupHostLiteral(host); err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			return dialer.DialContext(ctx, network, address)
		}
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve backup S3 endpoint: %w", err)
		}
		if len(addrs) == 0 {
			return nil, errors.New("backup S3 endpoint resolved to no addresses")
		}
		for _, addr := range addrs {
			if isBlockedBackupIP(addr.IP) {
				return nil, errors.New("backup S3 endpoint resolved to a disallowed address")
			}
		}
		var lastErr error
		for _, addr := range addrs {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many backup S3 redirects")
			}
			if req == nil || req.URL == nil {
				return errors.New("invalid backup S3 redirect")
			}
			_, err := validateBackupEndpoint(req.URL.String())
			return err
		},
	}
}

func isBlockedBackupIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, network := range backupBlockedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseBackupCIDRs(values []string) []*net.IPNet {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			panic("invalid backup SSRF CIDR: " + value)
		}
		result = append(result, network)
	}
	return result
}
