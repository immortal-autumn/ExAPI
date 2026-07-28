package repository

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/outbound"
	"github.com/stretchr/testify/require"
)

func TestConfigureUpstreamDestinationPolicyPinsDirectDial(t *testing.T) {
	var dialed string
	policy := outbound.Policy{
		Resolver: upstreamResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}),
		Dialer: upstreamDialerFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = address
			return nil, errors.New("stop before network")
		}),
	}
	transport := &http.Transport{}

	require.NoError(t, configureUpstreamDestinationPolicy(transport, nil, policy))
	_, err := transport.DialContext(context.Background(), "tcp", "upstream.example:443")
	require.ErrorContains(t, err, "stop before network")
	require.Equal(t, "93.184.216.34:443", dialed)
}

func TestConfigureUpstreamDestinationPolicyPreservesExplicitProxyDialer(t *testing.T) {
	proxyDialed := false
	transport := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		proxyDialed = true
		return nil, errors.New("proxy dial")
	}}
	proxyURL, err := url.Parse("socks5h://proxy.example:1080")
	require.NoError(t, err)

	require.NoError(t, configureUpstreamDestinationPolicy(transport, proxyURL, outbound.Policy{}))
	_, err = transport.DialContext(context.Background(), "tcp", "upstream.example:443")
	require.ErrorContains(t, err, "proxy dial")
	require.True(t, proxyDialed)
}

type upstreamResolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f upstreamResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

type upstreamDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f upstreamDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}
