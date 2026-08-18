package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newOllamaCloudUsageRepositoryTestClient(t *testing.T) (*dbent.Client, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return client, mock
}

func ollamaCloudUsageRepositoryAccount() *service.Account {
	return &service.Account{
		ID: 17, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "key", "base_url": "https://ollama.com"},
		Extra: map[string]any{
			service.OllamaCloudUsageSessionExtraKey:     "cipher:wos-session=secret",
			service.OllamaCloudUsageAutoRefreshExtraKey: true,
		},
	}
}

func TestUpdateOllamaCloudUsageSnapshotRowsAffectedZeroIsIdentityConflict(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, ollamaCloudUsageRepositoryAccount(), true,
		`"cipher:wos-session=secret"`, `true`, `null`)
	mock.ExpectExec(`(?s)`+regexp.QuoteMeta("UPDATE accounts")).
		WithArgs(sqlmock.AnyArg(), "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.UpdateOllamaCloudUsageSnapshot(context.Background(), ollamaCloudUsageRepositoryAccount(), &service.OllamaCloudUsageSnapshot{
		Status:        service.OllamaCloudUsageStatusOK,
		LastAttemptAt: time.Now(),
		NextRefreshAt: time.Now().Add(time.Hour),
	})

	require.ErrorIs(t, err, service.ErrOllamaCloudUsageIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectOllamaCloudUsageGroupLock(
	mock sqlmock.Sqlmock,
	account *service.Account,
	anchorMatches bool,
	sessionJSON, autoJSON, snapshotJSON string,
) {
	apiKey, _ := account.Credentials["api_key"].(string)
	credentials, _ := json.Marshal(normalizeJSONMap(account.Credentials))
	var proxyID any
	if account.ProxyID != nil {
		proxyID = *account.ProxyID
	}
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT")+`.*`+regexp.QuoteMeta("FOR NO KEY UPDATE")).
		WithArgs(apiKey, account.ID, account.Platform, account.Type, string(credentials), proxyID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "anchor_matches", "session", "auto_refresh", "snapshot"}).
			AddRow(account.ID, anchorMatches, sessionJSON, autoJSON, snapshotJSON))
}

func TestOllamaCloudUsageManagedWriteRejectsChangedProxyIdentity(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)` + regexp.QuoteMeta("SELECT protocol, host, port") + `.*` + regexp.QuoteMeta("FOR SHARE")).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"protocol", "host", "port", "username", "password", "status"}).
			AddRow("http", "new.example", 3128, "user", "pass", service.StatusActive))
	mock.ExpectRollback()

	account := ollamaCloudUsageRepositoryAccount()
	proxyID := int64(9)
	account.ProxyID = &proxyID
	account.Proxy = &service.Proxy{
		ID: proxyID, Protocol: "http", Host: "old.example", Port: 3128,
		Username: "user", Password: "pass", Status: service.StatusActive,
	}
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.SaveOllamaCloudUsageSession(context.Background(), account, "cipher:wos-session=replacement", true)

	require.ErrorIs(t, err, service.ErrOllamaCloudUsageIdentityChanged)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveAndDeleteOllamaCloudUsageSessionKeepCiphertextOutOfSQL(t *testing.T) {
	var capturedSQL []string
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		capturedSQL = append(capturedSQL, actualSQL)
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := newAccountRepositoryWithSQL(client, db, nil)
	account := ollamaCloudUsageRepositoryAccount()
	const replacement = "cipher:wos-session=browser-cookie-secret"

	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, account, true, `"cipher:wos-session=secret"`, `true`, `null`)
	mock.ExpectExec(`(?s)UPDATE accounts.*ollama_cloud_usage_session.*ollama_cloud_usage_auto_refresh.*ollama_cloud_usage_snapshot`).
		WithArgs(`{"ollama_cloud_usage_auto_refresh":true,"ollama_cloud_usage_session":"cipher:wos-session=browser-cookie-secret"}`, "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.SaveOllamaCloudUsageSession(context.Background(), account, replacement, true))

	account.Extra[service.OllamaCloudUsageSessionExtraKey] = replacement
	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, account, true, `"cipher:wos-session=browser-cookie-secret"`, `true`, `null`)
	mock.ExpectExec(`(?s)UPDATE accounts.*ollama_cloud_usage_session.*ollama_cloud_usage_auto_refresh.*ollama_cloud_usage_snapshot`).
		WithArgs(`{}`, "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.DeleteOllamaCloudUsageSession(context.Background(), account))

	require.NotEmpty(t, capturedSQL)
	for _, query := range capturedSQL {
		require.NotContains(t, query, "browser-cookie-secret")
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOllamaCloudBaseURLSQLRegexMatchesServiceSemantics(t *testing.T) {
	for _, baseURL := range []string{
		"https://ollama.com",
		"HTTPS://WWW.OLLAMA.COM:443/v1",
		"https://ollama.com/V1",
		"https://ollama.com/v1/",
		"https://ollama.com.evil.test/v1",
	} {
		t.Run(baseURL, func(t *testing.T) {
			matched, err := regexp.MatchString(ollamaCloudBaseURLRegexSQL, baseURL)
			require.NoError(t, err)
			account := ollamaCloudUsageRepositoryAccount()
			account.Credentials["base_url"] = baseURL
			require.Equal(t, service.IsOllamaCloudUsageAccount(account), matched)
		})
	}
}

func TestListOllamaCloudUsageGroupAccountsUsesOneStrictBatchQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var capturedSQL string
	mock.ExpectQuery("SELECT id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	repo := newAccountRepositoryWithSQL(nil, captureQuerySQL{db: db, captured: &capturedSQL}, nil)
	first := ollamaCloudUsageRepositoryAccount()
	second := ollamaCloudUsageRepositoryAccount()
	second.ID = 18
	second.Platform = service.PlatformAnthropic
	second.Credentials = map[string]any{"api_key": "key", "base_url": "https://www.ollama.com:443/v1"}

	accounts, err := repo.ListOllamaCloudUsageGroupAccounts(context.Background(), []*service.Account{first, second})

	require.NoError(t, err)
	require.Empty(t, accounts)
	query := normalizeSQLWhitespace(capturedSQL)
	require.Contains(t, query, "credentials ->> 'api_key' = ANY($1)")
	require.Contains(t, query, "platform IN ('openai', 'anthropic')")
	require.Contains(t, query, "jsonb_typeof(credentials -> 'api_key') = 'string'")
	require.Contains(t, query, ollamaCloudBaseURLMatchesSQL("credentials ->> 'base_url'"))
	require.NotContains(t, query, "~*")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListDueOllamaCloudUsageAccountsFiltersOrdersAndLimits(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	debounce := time.Minute
	maxWait := time.Hour
	var capturedSQL string
	mock.ExpectQuery("WITH eligible AS").
		WithArgs(now.UTC(), debounce.Seconds(), maxWait.Seconds(), 20, service.OllamaCloudUsageMinFetchInterval.Seconds()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "group_last_used_at"}))
	repo := newAccountRepositoryWithSQL(nil, captureQuerySQL{db: db, captured: &capturedSQL}, nil)

	accounts, err := repo.ListDueOllamaCloudUsageAccounts(context.Background(), now, debounce, maxWait, 20)

	require.NoError(t, err)
	require.Empty(t, accounts)
	normalized := normalizeSQLWhitespace(capturedSQL)
	for _, clause := range []string{
		"deleted_at IS NULL",
		"status = 'active'",
		"platform IN ('openai', 'anthropic')",
		"type = 'apikey'",
		ollamaCloudBaseURLMatchesSQL("credentials ->> 'base_url'"),
		"jsonb_typeof(extra -> 'ollama_cloud_usage_session') = 'string'",
		`extra @> '{"ollama_cloud_usage_auto_refresh": true}'::jsonb`,
		"MAX(last_used_at) AS group_last_used_at",
		"PARTITION BY api_key",
		"WHERE group_rank = 1",
		"LIMIT $4",
		"make_interval(secs => $2::double precision)",
		"make_interval(secs => $3::double precision)",
		// Minimum interval floor between successful fetches.
		"make_interval(secs => $5::double precision)",
		// jsonpath .datetime() only accepts the ISO-8601 "Z" designator from
		// PostgreSQL 17 on, and this service writes UTC timestamps. Without this
		// rewrite every parsed_* column is NULL on 14-16 and the due filter
		// collapses into its fail-open branch.
		`regexp_replace( regexp_replace( fetched_at, '(\.[0-9]{6})[0-9]+(Z|[+-][0-9]{2}:[0-9]{2})$', '\1\2' ), 'Z$', '+00:00' )`,
		"group_last_used_at > parsed_fetched_at::timestamptz",
		"group_last_used_at > parsed_last_attempt_at::timestamptz",
		"$1 >= activity_due_at",
		"COALESCE(parsed_next_refresh_at::timestamptz, '-infinity'::timestamptz)",
		"ORDER BY due_class, due_at NULLS FIRST, id",
	} {
		require.Contains(t, normalized, clause)
	}
	require.NotContains(t, normalized, "~*")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOllamaCloudUsageCredentialPatchChangesGroupIdentity(t *testing.T) {
	base := ollamaCloudUsageRepositoryAccount()
	for _, tt := range []struct {
		name  string
		patch map[string]any
		want  bool
	}{
		{name: "equivalent Ollama URL", patch: map[string]any{"base_url": "https://www.ollama.com:443/v1"}},
		{name: "ineligible base URL", patch: map[string]any{"base_url": "https://relay.example.com/v1"}, want: true},
		{name: "same API key", patch: map[string]any{"api_key": "key"}},
		{name: "changed API key", patch: map[string]any{"api_key": "replacement"}, want: true},
		{name: "unrelated credential", patch: map[string]any{"organization": "test"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			merged := copyJSONMap(base.Credentials)
			for key, value := range tt.patch {
				merged[key] = value
			}
			require.Equal(t, tt.want, ollamaCloudUsageCredentialPatchChangesGroupIdentity(base, merged, tt.patch))
		})
	}
}

func TestBulkUpdateOllamaCredentialCleanupUsesPerRowPlaintextDecision(t *testing.T) {
	for _, tt := range []struct {
		name        string
		credentials map[string]any
		wantCleanup bool
	}{
		{name: "equivalent base URL preserves managed state", credentials: map[string]any{"base_url": "https://www.ollama.com:443/v1"}},
		{name: "ineligible base URL clears managed state", credentials: map[string]any{"base_url": "https://relay.example.com/v1"}, wantCleanup: true},
		{name: "changed API key clears managed state", credentials: map[string]any{"api_key": "replacement"}, wantCleanup: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var updateQuery string
			matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
				if strings.HasPrefix(normalizeSQLWhitespace(actualSQL), "UPDATE accounts SET") {
					updateQuery = actualSQL
				}
				return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
			})
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			protector := mustAccountCredentialProtectorForTest(t)
			stored, err := protector.seal(17, map[string]any{"api_key": "key", "base_url": "https://ollama.com"})
			require.NoError(t, err)
			storedJSON, err := json.Marshal(stored)
			require.NoError(t, err)

			mock.ExpectQuery(`(?s)SELECT .* FROM "accounts".*WHERE .*"id" IN \(\$1\)`).
				WithArgs(int64(17)).
				WillReturnRows(updatedAccountRows(17, storedJSON, `{"ollama_cloud_usage_session":"cipher:session","ollama_cloud_usage_auto_refresh":true,"ollama_cloud_usage_snapshot":{"status":"ok"}}`))
			mock.ExpectQuery(`(?s)SELECT "account_groups".*WHERE "account_groups"\."account_id" IN \(\$1\)`).
				WithArgs(int64(17)).
				WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority", "created_at"}))
			mock.ExpectBegin()
			update := mock.ExpectExec(`(?s)UPDATE accounts SET credentials = CASE id .*WHERE id = ANY`).
				WillReturnResult(sqlmock.NewResult(0, 1))
			if tt.wantCleanup {
				update.WithArgs(int64(17), sqlmock.AnyArg(), `{17}`, `{17}`)
			} else {
				update.WithArgs(int64(17), sqlmock.AnyArg(), `{17}`)
			}
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()

			repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, protector)
			rows, err := repo.BulkUpdate(context.Background(), []int64{17}, service.AccountBulkUpdate{Credentials: tt.credentials})
			require.NoError(t, err)
			require.EqualValues(t, 1, rows)
			normalized := normalizeSQLWhitespace(updateQuery)
			require.NotContains(t, normalized, "credentials ->> 'base_url'", "sealed credential envelopes must not be inspected in SQL")
			if tt.wantCleanup {
				require.Contains(t, normalized, "id = ANY($3)")
				require.Contains(t, normalized, "- 'ollama_cloud_usage_session' - 'ollama_cloud_usage_auto_refresh' - 'ollama_cloud_usage_snapshot'")
			} else {
				require.NotContains(t, normalized, "ollama_cloud_usage_session")
				require.NotContains(t, normalized, "ollama_cloud_usage_snapshot")
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBulkUpdateOllamaCredentialCleanupSelectsOnlyChangedRows(t *testing.T) {
	var updateQuery string
	matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
		if strings.HasPrefix(normalizeSQLWhitespace(actualSQL), "UPDATE accounts SET") {
			updateQuery = actualSQL
		}
		return sqlmock.QueryMatcherRegexp.Match(expectedSQL, actualSQL)
	})
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	protector := mustAccountCredentialProtectorForTest(t)
	now := time.Now()
	rows := sqlmock.NewRows(dbaccount.Columns)
	for _, fixture := range []struct {
		id     int64
		apiKey string
	}{
		{id: 17, apiKey: "replacement"},
		{id: 18, apiKey: "old-key"},
	} {
		stored, sealErr := protector.seal(fixture.id, map[string]any{"api_key": fixture.apiKey, "base_url": "https://ollama.com"})
		require.NoError(t, sealErr)
		storedJSON, marshalErr := json.Marshal(stored)
		require.NoError(t, marshalErr)
		rows.AddRow(
			fixture.id, now, now, nil, "test", nil, service.PlatformOpenAI, service.AccountTypeAPIKey,
			storedJSON, []byte(`{"ollama_cloud_usage_session":"cipher:session","ollama_cloud_usage_auto_refresh":true,"ollama_cloud_usage_snapshot":{"status":"ok"}}`), nil, nil,
			1, nil, 1, 1.0, service.StatusActive, nil, nil, nil, false, true, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, service.QuotaDimensionGlobal,
		)
	}

	mock.ExpectQuery(`(?s)SELECT .* FROM "accounts".*WHERE .*"id" IN \(\$1, \$2\)`).
		WithArgs(int64(17), int64(18)).
		WillReturnRows(rows)
	mock.ExpectQuery(`(?s)SELECT "account_groups".*WHERE "account_groups"\."account_id" IN \(\$1, \$2\)`).
		WithArgs(int64(17), int64(18)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority", "created_at"}))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts SET credentials = CASE id .*WHERE id = ANY`).
		WithArgs(int64(17), sqlmock.AnyArg(), int64(18), sqlmock.AnyArg(), `{18}`, `{17,18}`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := newAccountRepositoryWithSQLAndProtector(client, db, nil, protector)
	updated, err := repo.BulkUpdate(context.Background(), []int64{17, 18}, service.AccountBulkUpdate{
		Credentials: map[string]any{"api_key": "replacement"},
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, updated)
	normalized := normalizeSQLWhitespace(updateQuery)
	require.Contains(t, normalized, "id = ANY($5)")
	require.Contains(t, normalized, "- 'ollama_cloud_usage_session' - 'ollama_cloud_usage_auto_refresh' - 'ollama_cloud_usage_snapshot'")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBulkUpdateOllamaProxyCleanupIsValueConditional(t *testing.T) {
	for _, tt := range []struct {
		name      string
		proxyID   int64
		condition string
	}{
		{name: "set proxy", proxyID: 9, condition: "proxy_id IS DISTINCT FROM $1"},
		{name: "clear proxy", proxyID: 0, condition: "proxy_id IS NOT NULL"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exec := &recordingSQLExecutor{result: rowsAffectedResult(1)}
			repo := newAccountRepositoryWithSQL(nil, exec, nil)
			rows, err := repo.BulkUpdate(context.Background(), []int64{17}, service.AccountBulkUpdate{ProxyID: &tt.proxyID})
			require.NoError(t, err)
			require.EqualValues(t, 1, rows)
			require.NotEmpty(t, exec.execQueries)
			query := normalizeSQLWhitespace(exec.execQueries[0])
			require.Contains(t, query, tt.condition)
			require.Contains(t, query, "platform IN ('openai', 'anthropic') AND type = 'apikey'")
			require.Contains(t, query, "- 'ollama_cloud_usage_snapshot'")
			require.NotContains(t, query, "ollama_cloud_usage_session")
			require.NotContains(t, query, "ollama_cloud_usage_auto_refresh")
		})
	}
}

func TestUpdateCredentialsIdentityChangeClearsAllOllamaManagedExtra(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	protector := mustAccountCredentialProtectorForTest(t)
	stored, err := protector.seal(17, map[string]any{"api_key": "old-key", "base_url": "https://ollama.com"})
	require.NoError(t, err)
	storedJSON, err := json.Marshal(stored)
	require.NoError(t, err)
	mock.ExpectQuery(`(?s)SELECT .* FROM "accounts".*WHERE .*"id" = \$1`).
		WithArgs(int64(17)).
		WillReturnRows(updatedAccountRows(17, storedJSON, `{"ollama_cloud_usage_session":"cipher:session","ollama_cloud_usage_auto_refresh":true,"ollama_cloud_usage_snapshot":{"status":"ok"}}`))
	mock.ExpectQuery(`(?s)SELECT "account_groups".*WHERE "account_groups"\."account_id" IN \(\$1\)`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority", "created_at"}))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*WHEN \$4.*ollama_cloud_usage_session.*ollama_cloud_usage_auto_refresh.*ollama_cloud_usage_snapshot`).
		WithArgs(sqlmock.AnyArg(), int64(17), true, true).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQLAndProtector(client, nil, nil, protector)

	err = repo.UpdateCredentials(context.Background(), 17, map[string]any{
		"api_key": "new-key", "base_url": "https://ollama.com",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDisableOllamaCloudUsageAutoRefreshUsesGroupIdentityCAS(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	account := ollamaCloudUsageRepositoryAccount()
	mock.ExpectBegin()
	expectOllamaCloudUsageGroupLock(mock, account, true, `"cipher:wos-session=secret"`, `true`, `null`)
	mock.ExpectExec(`(?s)UPDATE accounts.*ollama_cloud_usage_auto_refresh`).
		WithArgs(`{"ollama_cloud_usage_auto_refresh":false,"ollama_cloud_usage_session":"cipher:wos-session=secret"}`, "key", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQL(client, nil, nil)

	err := repo.DisableOllamaCloudUsageAutoRefresh(context.Background(), account)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Ollama 清理分支必须带顶层 credentials DISTINCT 守卫：没有它，非 Ollama 的
// openai/anthropic apikey 账号在凭证未变化的持久化上也会误清探测快照。
func TestUpdateCredentialsCleanupBranchRequiresChangedCredentials(t *testing.T) {
	client, mock := newOllamaCloudUsageRepositoryTestClient(t)
	protector := mustAccountCredentialProtectorForTest(t)
	stored, err := protector.seal(17, map[string]any{"api_key": "same-key", "base_url": "https://relay.example.com/v1"})
	require.NoError(t, err)
	storedJSON, err := json.Marshal(stored)
	require.NoError(t, err)
	mock.ExpectQuery(`(?s)SELECT .* FROM "accounts".*WHERE .*"id" = \$1`).
		WithArgs(int64(17)).
		WillReturnRows(updatedAccountRows(17, storedJSON, `{"ollama_cloud_usage_snapshot":{"status":"stale"}}`))
	mock.ExpectQuery(`(?s)SELECT "account_groups".*WHERE "account_groups"\."account_id" IN \(\$1\)`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority", "created_at"}))
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)UPDATE accounts.*WHEN \$4.*ollama_cloud_usage_session`).
		WithArgs(sqlmock.AnyArg(), int64(17), false, false).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO scheduler_outbox")).
		WithArgs(service.SchedulerOutboxEventAccountChanged, int64(17), nil, nil, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	repo := newAccountRepositoryWithSQLAndProtector(client, nil, nil, protector)

	err = repo.UpdateCredentials(context.Background(), 17, map[string]any{
		"api_key": "same-key", "base_url": "https://relay.example.com/v1",
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
