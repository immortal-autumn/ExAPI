package outbound

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type resolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f resolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

type dialerFunc func(context.Context, string, string) (net.Conn, error)

func (f dialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func TestPolicyDialContextPinsResolvedPublicAddress(t *testing.T) {
	var resolvedHost, dialedAddress string
	policy := Policy{
		Resolver: resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			resolvedHost = host
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}),
		Dialer: dialerFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialedAddress = address
			client, server := net.Pipe()
			t.Cleanup(func() { _ = client.Close() })
			_ = server.Close()
			return client, nil
		}),
	}

	conn, err := policy.DialContext(context.Background(), "tcp", "moderation.example:443")
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Equal(t, "moderation.example", resolvedHost)
	require.Equal(t, "93.184.216.34:443", dialedAddress)
}

func TestPolicyDialContextRejectsMixedPublicAndPrivateAnswersBeforeDial(t *testing.T) {
	dialed := false
	policy := Policy{
		Resolver: resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{
				{IP: net.ParseIP("93.184.216.34")},
				{IP: net.ParseIP("127.0.0.1")},
			}, nil
		}),
		Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("unexpected dial")
		}),
	}

	conn, err := policy.DialContext(context.Background(), "tcp", "rebinding.example:443")
	require.Nil(t, conn)
	require.ErrorContains(t, err, "disallowed")
	require.False(t, dialed)
}

func TestPolicyDialContextRevalidatesEveryNewSocket(t *testing.T) {
	lookups := 0
	dials := 0
	policy := Policy{
		Resolver: resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			lookups++
			if lookups == 1 {
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
			}
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}),
		Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
			dials++
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		}),
	}

	first, err := policy.DialContext(context.Background(), "tcp", "rebinding.example:443")
	require.NoError(t, err)
	require.NoError(t, first.Close())

	second, err := policy.DialContext(context.Background(), "tcp", "rebinding.example:443")
	require.Nil(t, second)
	require.ErrorContains(t, err, "disallowed")
	require.Equal(t, 2, lookups)
	require.Equal(t, 1, dials)
}

func TestPolicyRedirectCheckerRevalidatesDestination(t *testing.T) {
	policy := Policy{}
	check := policy.RedirectChecker(false, 10)

	publicReq, err := http.NewRequest(http.MethodGet, "https://example.com/next", nil)
	require.NoError(t, err)
	require.NoError(t, check(publicReq, []*http.Request{{}}))

	privateReq, err := http.NewRequest(http.MethodGet, "https://127.0.0.1/internal", nil)
	require.NoError(t, err)
	require.ErrorContains(t, check(privateReq, []*http.Request{{}}), "disallowed")

	insecureReq, err := http.NewRequest(http.MethodGet, "http://example.com/plaintext", nil)
	require.NoError(t, err)
	require.ErrorContains(t, check(insecureReq, []*http.Request{{}}), "https")
}

func TestConfigureTransportDirectPinsWhileTrustedProxyKeepsProxyDialer(t *testing.T) {
	policyDialer := dialerFunc(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("policy dialer")
	})
	policy := Policy{Dialer: policyDialer}

	direct := &http.Transport{}
	require.NoError(t, policy.ConfigureTransport(direct, DirectResolution))
	require.NotNil(t, direct.DialContext)
	_, err := direct.DialContext(context.Background(), "tcp", "example.com:443")
	require.ErrorContains(t, err, "policy dialer")

	proxyDialerCalled := false
	proxied := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		proxyDialerCalled = true
		return nil, errors.New("trusted proxy dialer")
	}}
	require.NoError(t, policy.ConfigureTransport(proxied, TrustedProxyResolution))
	_, err = proxied.DialContext(context.Background(), "tcp", "proxy.example:1080")
	require.ErrorContains(t, err, "trusted proxy dialer")
	require.True(t, proxyDialerCalled)
}

func TestPolicyRejectsSpecialUseLiteralDestinations(t *testing.T) {
	policy := PublicPolicy()
	for _, rawURL := range []string{
		"https://127.0.0.1",
		"https://10.0.0.1",
		"https://169.254.169.254/latest/meta-data",
		"https://168.63.129.16/",
		"https://192.88.99.2/",
		"https://[::ffff:192.88.99.2]/",
		"https://[::1]",
		"https://[fc00::1]",
		"https://[fe80::1]",
		"https://[64:ff9b::7f00:1]",
		"https://[64:ff9b::a00:1]",
		"https://[64:ff9b::a9fe:a9fe]",
		"https://[64:ff9b:1::7f00:1]",
		"https://[64:ff9b:1::a00:1]",
		"https://[64:ff9b:1::a9fe:a9fe]",
		"https://[100::1]",
		"https://[100:0:0:1::1]",
		"https://[2001::1]",
		"https://[2001:2::1]",
		"https://[2002:0a00:0001::1]",
		"https://[3ffe::1]",
		"https://[3fff::1]",
		"https://[4000::1]",
		"https://[5f00::1]",
		"https://[8000::1]",
		"https://[c000::1]",
		"https://[fec0::1]",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := policy.ValidateURL(rawURL, false)
			require.ErrorContains(t, err, "disallowed")
		})
	}
}

func TestPolicyRejectsSpecialUseResolvedDestinationsBeforeDial(t *testing.T) {
	for _, rawIP := range []string{
		"10.0.0.1",
		"169.254.169.254",
		"168.63.129.16",
		"192.88.99.2",
		"::ffff:192.88.99.2",
		"fc00::1",
		"fe80::1",
		"64:ff9b::7f00:1",
		"64:ff9b::a00:1",
		"64:ff9b::a9fe:a9fe",
		"64:ff9b:1::7f00:1",
		"64:ff9b:1::a00:1",
		"64:ff9b:1::a9fe:a9fe",
		"100::1",
		"100:0:0:1::1",
		"2001::1",
		"2001:2::1",
		"2002:0a00:0001::1",
		"3ffe::1",
		"3fff::1",
		"4000::1",
		"5f00::1",
		"8000::1",
		"c000::1",
		"fec0::1",
	} {
		t.Run(rawIP, func(t *testing.T) {
			dialed := false
			policy := Policy{
				Resolver: resolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
					return []net.IPAddr{{IP: net.ParseIP(rawIP)}}, nil
				}),
				Dialer: dialerFunc(func(context.Context, string, string) (net.Conn, error) {
					dialed = true
					return nil, errors.New("unexpected dial")
				}),
			}

			conn, err := policy.DialContext(context.Background(), "tcp", "moderation.example:443")
			require.Nil(t, conn)
			require.ErrorContains(t, err, "disallowed")
			require.False(t, dialed)
		})
	}
}

func TestPolicyAllowsMappedPublicIPv4Literal(t *testing.T) {
	_, err := PublicPolicy().ValidateURL("https://[::ffff:93.184.216.34]", false)
	require.NoError(t, err)
}
