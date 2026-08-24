package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/outbound"
	"github.com/stretchr/testify/require"
)

type testResolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f testResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

type testDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f testDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestBuildClientPinsValidatedAddressAtDialTime(t *testing.T) {
	var dialed string
	policy := outbound.Policy{
		Resolver: testResolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			require.Equal(t, "api.example.test", host)
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}),
		Dialer: testDialerFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = address
			return nil, errors.New("dial sentinel")
		}),
	}

	client, err := buildClientWithPolicy(Options{ValidateResolvedIP: true}, policy)
	require.NoError(t, err)
	_, err = client.Get("https://api.example.test/resource")
	require.ErrorContains(t, err, "dial sentinel")
	require.Equal(t, "93.184.216.34:443", dialed)
}

func TestBuildClientRejectsDNSRebindingBeforeDial(t *testing.T) {
	dialed := false
	policy := outbound.Policy{
		Resolver: testResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("93.184.216.34")},
				{IP: net.ParseIP("127.0.0.1")},
			}, nil
		}),
		Dialer: testDialerFunc(func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("unexpected dial")
		}),
	}

	client, err := buildClientWithPolicy(Options{ValidateResolvedIP: true}, policy)
	require.NoError(t, err)
	_, err = client.Get("https://rebind.example.test/resource")
	require.ErrorContains(t, err, "disallowed")
	require.False(t, dialed)
}

func TestBuildClientPreservesConfiguredProxyResolution(t *testing.T) {
	policyResolverCalled := false
	policy := outbound.Policy{
		Resolver: testResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			policyResolverCalled = true
			return nil, errors.New("policy resolver must not resolve proxy destinations")
		}),
	}

	client, err := buildClientWithPolicy(Options{
		ProxyURL:           "http://127.0.0.1:1",
		ValidateResolvedIP: true,
	}, policy)
	require.NoError(t, err)
	_, err = client.Get("https://provider.example.test/resource")
	require.Error(t, err)
	require.False(t, policyResolverCalled)
}

func TestRedirectCheckerRejectsHTTPSDowngradeAndPrivatePivot(t *testing.T) {
	check := redirectChecker(outbound.PublicPolicy(), 5)
	original, err := http.NewRequest(http.MethodGet, "https://api.example.test/start", nil)
	require.NoError(t, err)

	downgrade, err := http.NewRequest(http.MethodGet, "http://api.example.test/next", nil)
	require.NoError(t, err)
	require.ErrorContains(t, check(downgrade, []*http.Request{original}), "downgrade")

	private, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/internal", nil)
	require.NoError(t, err)
	require.ErrorContains(t, check(private, []*http.Request{original}), "disallowed")
}

func TestCompatibilityClientKeepsDefaultRedirectBehavior(t *testing.T) {
	client, err := buildClientWithPolicy(Options{ValidateResolvedIP: false}, outbound.PublicPolicy())
	require.NoError(t, err)
	require.Nil(t, client.CheckRedirect)
}
