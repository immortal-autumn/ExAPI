package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestUpdateUpstreamBillingProbeSnapshotMatchesDecryptedCredentialIdentity(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	expectLockedAccountCredentials(mock, int64(17), `{"api_key":"sk-test"}`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*`+regexp.QuoteMeta("AND proxy_id IS NOT DISTINCT FROM $5")+`.*`+regexp.QuoteMeta("COALESCE(extra -> 'upstream_billing_probe', 'null'::jsonb) = $6::jsonb")).
		WithArgs(sqlmock.AnyArg(), int64(17), service.PlatformOpenAI, service.AccountTypeAPIKey, nil, "null", "null", "null", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{ID: 17, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-test"}}

	err := repo.UpdateUpstreamBillingProbeSnapshot(context.Background(), account, &service.UpstreamBillingProbeSnapshot{Status: service.UpstreamBillingProbeStatusOK}, nil)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUpstreamBillingProbeSnapshotRejectsChangedCredentialsBeforeMutation(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	expectLockedAccountCredentials(mock, int64(17), `{"api_key":"rotated"}`)
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{ID: 17, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "observed"}}

	err := repo.UpdateUpstreamBillingProbeSnapshot(context.Background(), account, &service.UpstreamBillingProbeSnapshot{Status: service.UpstreamBillingProbeStatusOK}, nil)
	require.ErrorIs(t, err, service.ErrUpstreamBillingProbeIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUpstreamBillingProbeSnapshotRejectsChangedProxyIdentity(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT protocol, host, port") + `.*` + regexp.QuoteMeta("FOR SHARE")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"protocol", "host", "port", "username", "password", "status"}).
			AddRow("http", "new.example", 3128, "user", "pass", service.StatusActive))
	mock.ExpectRollback()

	proxyID := int64(9)
	account := &service.Account{
		ID: 17, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"}, ProxyID: &proxyID,
		Proxy: &service.Proxy{ID: proxyID, Protocol: "http", Host: "old.example", Port: 3128, Username: "user", Password: "pass", Status: service.StatusActive},
	}
	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))

	err := repo.UpdateUpstreamBillingProbeSnapshot(context.Background(), account, &service.UpstreamBillingProbeSnapshot{Status: service.UpstreamBillingProbeStatusOK}, nil)
	require.ErrorIs(t, err, service.ErrUpstreamBillingProbeIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUpstreamBillingProbeSnapshotRollsBackWhenOutboxFails(t *testing.T) {
	db, mock, client := probeCASTestDB(t)
	mock.ExpectBegin()
	expectLockedAccountCredentials(mock, int64(18), `{"api_key":"sk-test"}`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")+`.*`+regexp.QuoteMeta("AND proxy_id IS NOT DISTINCT FROM $5")).
		WithArgs(sqlmock.AnyArg(), int64(18), service.PlatformOpenAI, service.AccountTypeAPIKey, nil, "null", "null", "null", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).WillReturnError(errors.New("outbox failed"))
	mock.ExpectRollback()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, mustAccountCredentialProtectorForTest(t))
	account := &service.Account{ID: 18, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-test"}}

	err := repo.UpdateUpstreamBillingProbeSnapshot(context.Background(), account, &service.UpstreamBillingProbeSnapshot{Status: service.UpstreamBillingProbeStatusOK}, nil)
	require.EqualError(t, err, "outbox failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func probeCASTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *dbent.Client) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close(); _ = db.Close() })
	return db, mock, client
}

func expectLockedAccountCredentials(mock sqlmock.Sqlmock, id int64, credentials string) {
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT credentials") + `.*` + regexp.QuoteMeta("FOR UPDATE")).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"credentials"}).AddRow([]byte(credentials)))
}
