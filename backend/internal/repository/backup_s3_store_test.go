package repository

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNewS3BackupStoreFactoryRejectsUnsafeEndpoints(t *testing.T) {
	factory := NewS3BackupStoreFactory()
	for _, endpoint := range []string{
		"http://example.com",
		"https://127.0.0.1",
		"https://169.254.169.254",
		"https://localhost",
		"https://224.0.0.1",
		"https://240.0.0.1",
		"https://198.18.0.1",
		"https://[ff00::1]",
	} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := factory(context.Background(), &service.BackupS3Config{
				Endpoint:        endpoint,
				Region:          "auto",
				Bucket:          "backup",
				AccessKeyID:     "test-access",
				SecretAccessKey: "test-secret",
			})
			require.Error(t, err)
		})
	}
}

func TestNewS3BackupStoreFactoryIgnoresSDKEndpointOverrides(t *testing.T) {
	t.Setenv("AWS_ENDPOINT_URL", "http://127.0.0.1:9000")
	t.Setenv("AWS_ENDPOINT_URL_S3", "http://169.254.169.254")
	factory := NewS3BackupStoreFactory()
	store, err := factory(context.Background(), &service.BackupS3Config{
		Region:          "us-east-1",
		Bucket:          "backup",
		AccessKeyID:     "test-access",
		SecretAccessKey: "test-secret",
	})
	require.NoError(t, err)
	actual, ok := store.(*S3BackupStore)
	require.True(t, ok)
	require.Nil(t, actual.client.Options().BaseEndpoint)
}

func TestNewS3BackupStoreFactoryAcceptsPublicHTTPSEndpoint(t *testing.T) {
	factory := NewS3BackupStoreFactory()
	store, err := factory(context.Background(), &service.BackupS3Config{
		Endpoint:        "https://s3.example.com",
		Region:          "auto",
		Bucket:          "backup",
		AccessKeyID:     "test-access",
		SecretAccessKey: "test-secret",
	})
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestBackupHTTPClientDisablesEnvironmentProxy(t *testing.T) {
	client := newBackupHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy)
}

func TestBackupHTTPClientRejectsPrivateDialTargets(t *testing.T) {
	client := newBackupHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	for _, address := range []string{"127.0.0.1:443", "169.254.169.254:443", "[::1]:443"} {
		conn, err := transport.DialContext(context.Background(), "tcp", address)
		if conn != nil {
			_ = conn.Close()
		}
		require.Error(t, err, address)
	}
}

func TestBackupHTTPClientRejectsUnsafeRedirects(t *testing.T) {
	client := newBackupHTTPClient()
	for _, target := range []string{"http://example.com/object", "https://127.0.0.1/object"} {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		require.NoError(t, err)
		require.Error(t, client.CheckRedirect(req, nil), target)
	}
}

func TestBackupHTTPClientLimitsRedirects(t *testing.T) {
	client := newBackupHTTPClient()
	req, err := http.NewRequest(http.MethodGet, "https://s3.example.com/object", nil)
	require.NoError(t, err)
	via := make([]*http.Request, 10)
	require.Error(t, client.CheckRedirect(req, via))
}
