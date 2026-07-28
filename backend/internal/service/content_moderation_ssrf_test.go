package service

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/outbound"
	"github.com/stretchr/testify/require"
)

func TestContentModerationConfigRejectsPrivateBaseURL(t *testing.T) {
	svc := NewContentModerationService(nil, nil, nil, nil, nil, nil, nil)
	cfg := defaultContentModerationConfig()
	cfg.BaseURL = "https://127.0.0.1:8443"

	err := svc.validateConfig(context.Background(), cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Base URL")
}

func TestContentModerationHTTPClientPinsDNSAndDisablesEnvironmentProxy(t *testing.T) {
	var dialed string
	policy := outbound.Policy{
		Resolver: contentModerationResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}),
		Dialer: contentModerationDialerFunc(func(_ context.Context, _ string, address string) (net.Conn, error) {
			dialed = address
			return nil, errors.New("stop before network")
		}),
	}
	client := newContentModerationHTTPClient(policy)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy)

	_, err := transport.DialContext(context.Background(), "tcp", "moderation.example:443")
	require.ErrorContains(t, err, "stop before network")
	require.Equal(t, "93.184.216.34:443", dialed)
}

func TestContentModerationHTTPClientRejectsDestinationBeforeSendingCredentialOrContent(t *testing.T) {
	dialed := false
	policy := outbound.Policy{
		Resolver: contentModerationResolverFunc(func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}),
		Dialer: contentModerationDialerFunc(func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errors.New("unexpected dial")
		}),
	}
	client := newContentModerationHTTPClient(policy)
	req, err := http.NewRequest(http.MethodPost, "https://moderation.example/v1/moderations", bytes.NewBufferString(`{"input":"sensitive gateway content"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer moderation-secret")

	resp, err := client.Do(req)
	require.Nil(t, resp)
	require.ErrorContains(t, err, "disallowed")
	require.False(t, dialed, "no socket may open before the destination is accepted")
}

type contentModerationResolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (f contentModerationResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

type contentModerationDialerFunc func(context.Context, string, string) (net.Conn, error)

func (f contentModerationDialerFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}
