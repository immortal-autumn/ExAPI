package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpdateAccountTestProbeMatchesCurrentIdentity(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	expectLockedAccountCredentials(mock, int64(22), `{"access_token":"rotated-token","project_id":"project"}`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*`+regexp.QuoteMeta("AND proxy_id IS NOT DISTINCT FROM $5")).
		WithArgs(sqlmock.AnyArg(), int64(22), service.PlatformAntigravity, service.AccountTypeOAuth, nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{
		ID:       22,
		Platform: service.PlatformAntigravity,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token",
			"project_id":   "project",
		},
	}

	err := repo.UpdateAccountTestProbe(context.Background(), account, map[string]any{"status": service.AccountTestProbeStatusSuccess})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAccountTestProbeRejectsChangedCredentials(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	expectLockedAccountCredentials(mock, int64(22), `{"access_token":"rotated","project_id":"different-project"}`)
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{
		ID:       22,
		Platform: service.PlatformAntigravity,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stale",
			"project_id":   "project",
		},
	}

	err := repo.UpdateAccountTestProbe(context.Background(), account, map[string]any{"status": service.AccountTestProbeStatusFailed})
	require.ErrorIs(t, err, service.ErrAccountTestProbeIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAccountTestProbeLocksProxyBeforeAccount(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT protocol, host, port") + `.*` + regexp.QuoteMeta("FOR SHARE")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"protocol", "host", "port", "username", "password", "status"}).
			AddRow("http", "proxy.example", 3128, "user", "pass", service.StatusActive))
	expectLockedAccountCredentials(mock, int64(22), `{"access_token":"token","project_id":"project"}`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*`+regexp.QuoteMeta("AND proxy_id IS NOT DISTINCT FROM $5")).
		WithArgs(sqlmock.AnyArg(), int64(22), service.PlatformAntigravity, service.AccountTypeOAuth, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	proxyID := int64(9)
	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{
		ID:          22,
		Platform:    service.PlatformAntigravity,
		Type:        service.AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token", "project_id": "project"},
		ProxyID:     &proxyID,
		Proxy: &service.Proxy{
			ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 3128,
			Username: "user", Password: "pass", Status: service.StatusActive,
		},
	}

	err := repo.UpdateAccountTestProbe(context.Background(), account, map[string]any{"status": service.AccountTestProbeStatusSuccess})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
