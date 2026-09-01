//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateProxyOmittedPatchFieldsPreserveExistingValues(t *testing.T) {
	expiresAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	backupID := int64(12)
	repo := &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy: &Proxy{
			ID:             9,
			Name:           "primary",
			Protocol:       "http",
			Host:           "old.example",
			Port:           8080,
			Username:       "old-user",
			Password:       "old-secret",
			Status:         StatusActive,
			ExpiresAt:      &expiresAt,
			FallbackMode:   FallbackModeProxy,
			BackupProxyID:  &backupID,
			ExpiryWarnDays: 7,
		},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	updated, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{Host: "new.example"})
	require.NoError(t, err)
	require.Equal(t, "new.example", updated.Host)
	require.Equal(t, "old-user", updated.Username)
	require.Equal(t, "old-secret", updated.Password)
	require.Equal(t, &expiresAt, updated.ExpiresAt)
	require.Equal(t, FallbackModeProxy, updated.FallbackMode)
	require.Equal(t, &backupID, updated.BackupProxyID)
	require.Equal(t, 7, updated.ExpiryWarnDays)
}

func TestAdminService_UpdateProxyExplicitNullableFieldsCanBeCleared(t *testing.T) {
	expiresAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	backupID := int64(12)
	repo := &updatingProxyRepoStub{
		proxyRepoStub: &proxyRepoStub{},
		proxy: &Proxy{
			ID:             9,
			Name:           "primary",
			Protocol:       "http",
			Host:           "proxy.example",
			Port:           8080,
			Username:       "old-user",
			Password:       "old-secret",
			Status:         StatusActive,
			ExpiresAt:      &expiresAt,
			FallbackMode:   FallbackModeProxy,
			BackupProxyID:  &backupID,
			ExpiryWarnDays: 7,
		},
	}
	svc := &adminServiceImpl{proxyRepo: repo}

	updated, err := svc.UpdateProxy(context.Background(), 9, &UpdateProxyInput{
		UsernameSet:       true,
		PasswordSet:       true,
		ExpiresAtSet:      true,
		FallbackMode:      FallbackModeNone,
		FallbackModeSet:   true,
		BackupProxyIDSet:  true,
		ExpiryWarnDaysSet: true,
	})
	require.NoError(t, err)
	require.Empty(t, updated.Username)
	require.Empty(t, updated.Password)
	require.Nil(t, updated.ExpiresAt)
	require.Equal(t, FallbackModeNone, updated.FallbackMode)
	require.Nil(t, updated.BackupProxyID)
	require.Zero(t, updated.ExpiryWarnDays)
}
