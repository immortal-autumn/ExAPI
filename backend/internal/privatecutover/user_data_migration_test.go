package privatecutover

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUserScopedDataPolicyIsSafeAndVersioned(t *testing.T) {
	require.NoError(t, validateUserScopedDataPolicy())
	require.Equal(t, UserScopedDataPolicySchemaVersion, 1)
	require.GreaterOrEqual(t, len(UserScopedDataPolicy), 20)
	require.Contains(t, UserScopedDataPolicy, UserScopedDataPolicyEntry{
		Table: "user_external_identities", Column: "user_id", Disposition: UserDataDeleteCustomerRows,
	})
}

func TestUserScopedDataPolicyIdentityMappingsMatchTableShapes(t *testing.T) {
	want := map[string]userDataIdentityKind{
		"prompt_audit_events.user_id":          userDataIdentityID,
		"user_allowed_groups.user_id":          userDataIdentityUserGroup,
		"user_group_rate_multipliers.user_id":  userDataIdentityUserGroup,
		"usage_dashboard_hourly_users.user_id": userDataIdentityHourlyUser,
		"usage_dashboard_daily_users.user_id":  userDataIdentityDailyUser,
		"user_affiliates.user_id":              userDataIdentityUserColumn,
		"user_affiliates.inviter_id":            userDataIdentityUserColumn,
		"passkey_user_handles.user_id":          userDataIdentityUserColumn,
	}
	for _, entry := range userScopedDataPolicy {
		key := entry.public.Table + "." + entry.public.Column
		if expected, ok := want[key]; ok {
			require.Equal(t, expected, entry.identity, "identity mapping for %s", key)
		}
	}

	for key, expected := range want {
		var matched *userScopedDataPolicyEntry
		for index := range userScopedDataPolicy {
			entry := &userScopedDataPolicy[index]
			if entry.public.Table+"."+entry.public.Column == key {
				matched = entry
				break
			}
		}
		require.NotNil(t, matched, "policy entry %s", key)
		require.Equal(t, expected, matched.identity, "identity mapping for %s", key)
		identitySQL, err := userScopedIdentitySQL(*matched)
		require.NoError(t, err)
		switch key {
		case "prompt_audit_events.user_id":
			require.Equal(t, `"id"`, identitySQL)
		case "user_allowed_groups.user_id", "user_group_rate_multipliers.user_id":
			require.Equal(t, `"user_id"::text || ':' || "group_id"::text`, identitySQL)
		case "usage_dashboard_hourly_users.user_id":
			require.Equal(t, `"bucket_start"::text || ':' || "user_id"::text`, identitySQL)
		case "usage_dashboard_daily_users.user_id":
			require.Equal(t, `"bucket_date"::text || ':' || "user_id"::text`, identitySQL)
		case "user_affiliates.user_id", "user_affiliates.inviter_id", "passkey_user_handles.user_id":
			require.Equal(t, `"user_id"`, identitySQL)
		}
	}

	for _, entry := range userScopedDataPolicy {
		if entry.public.Table == "ops_ingress_reject_aggregates" && entry.public.Column == "user_id" {
			require.True(t, entry.keepGlobal)
			return
		}
	}
	t.Fatal("missing policy entry ops_ingress_reject_aggregates.user_id")
}

func TestMigrateUserScopedDataSkipsMissingTableAndColumn(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	policy := []userScopedDataPolicyEntry{
		{public: UserScopedDataPolicyEntry{Table: "legacy_only", Column: "user_id", Disposition: UserDataDeleteCustomerRows}, identity: userDataIdentityID},
		{public: UserScopedDataPolicyEntry{Table: "partial_table", Column: "user_id", Disposition: UserDataNullHistoricalRef}, identity: userDataIdentityID},
	}
	mock.ExpectQuery(`(?s)FROM pg_catalog\.pg_class`).WithArgs("legacy_only").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)FROM pg_catalog\.pg_class`).WithArgs("partial_table").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`(?s)FROM information_schema\.columns`).WithArgs("partial_table", "user_id").
		WillReturnRows(sqlmock.NewRows([]string{"exists", "is_nullable"}).AddRow(false, "NO"))

	evidence, err := migrateUserScopedDataWithPolicy(context.Background(), tx, 42, policy)
	require.NoError(t, err)
	require.Equal(t, []UserScopedDataEvidence{
		{Table: "legacy_only", Column: "user_id", Disposition: UserDataDeleteCustomerRows, Status: "skipped_missing_table"},
		{Table: "partial_table", Column: "user_id", Disposition: UserDataNullHistoricalRef, Status: "skipped_missing_column"},
	}, evidence)
	mock.ExpectCommit()
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMigrateUserScopedDataRejectsNonNullableHistoricalReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	policy := []userScopedDataPolicyEntry{{
		public:   UserScopedDataPolicyEntry{Table: "audit_logs", Column: "actor_user_id", Disposition: UserDataNullHistoricalRef},
		identity: userDataIdentityID,
	}}
	mock.ExpectQuery(`(?s)FROM pg_catalog\.pg_class`).WithArgs("audit_logs").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`(?s)FROM information_schema\.columns`).WithArgs("audit_logs", "actor_user_id").
		WillReturnRows(sqlmock.NewRows([]string{"exists", "is_nullable"}).AddRow(true, "NO"))

	evidence, err := migrateUserScopedDataWithPolicy(context.Background(), tx, 42, policy)
	require.Nil(t, evidence)
	require.ErrorContains(t, err, "not nullable")
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMigrateUserScopedDataAppliesMigrateNullAndDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	checksum := strings.Repeat("a", 32)
	policy := []userScopedDataPolicyEntry{
		{public: UserScopedDataPolicyEntry{Table: "api_keys", Column: "user_id", Disposition: UserDataMigrateToOperator}, identity: userDataIdentityID},
		{public: UserScopedDataPolicyEntry{Table: "audit_logs", Column: "actor_user_id", Disposition: UserDataNullHistoricalRef}, identity: userDataIdentityID},
		{public: UserScopedDataPolicyEntry{Table: "orphan_allowed_groups_audit", Column: "user_id", Disposition: UserDataDeleteCustomerRows}, identity: userDataIdentityID},
	}
	for _, entry := range policy {
		mock.ExpectQuery(`(?s)FROM pg_catalog\.pg_class`).WithArgs(entry.public.Table).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		nullable := "NO"
		if entry.public.Disposition == UserDataNullHistoricalRef {
			nullable = "YES"
		}
		mock.ExpectQuery(`(?s)FROM information_schema\.columns`).WithArgs(entry.public.Table, entry.public.Column).
			WillReturnRows(sqlmock.NewRows([]string{"exists", "is_nullable"}).AddRow(true, nullable))
		mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM`).
			WillReturnRows(sqlmock.NewRows([]string{"count", "checksum"}).AddRow(int64(3), checksum))
		switch entry.public.Disposition {
		case UserDataMigrateToOperator:
			mock.ExpectExec(`UPDATE "api_keys" SET "user_id" = \$1 WHERE`).WithArgs(int64(42)).
				WillReturnResult(sqlmock.NewResult(0, 2))
		case UserDataNullHistoricalRef:
			mock.ExpectExec(`UPDATE "audit_logs" SET "actor_user_id" = NULL WHERE`).WithArgs(int64(42)).
				WillReturnResult(sqlmock.NewResult(0, 1))
		case UserDataDeleteCustomerRows:
			mock.ExpectExec(`DELETE FROM "orphan_allowed_groups_audit" WHERE`).WithArgs(int64(42)).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM`).
			WillReturnRows(sqlmock.NewRows([]string{"count", "checksum"}).AddRow(int64(3), checksum))
	}

	evidence, err := migrateUserScopedDataWithPolicy(context.Background(), tx, 42, policy)
	require.NoError(t, err)
	require.Len(t, evidence, 3)
	require.Equal(t, int64(2), evidence[0].ChangedRows)
	require.Equal(t, int64(1), evidence[1].ChangedRows)
	require.Equal(t, int64(1), evidence[2].ChangedRows)
	require.NoError(t, validateUserScopedDataEvidenceWithPolicy(evidence, policy))
	mock.ExpectCommit()
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserScopedDataRejectsUntrustedIdentifiers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, err = userScopedTableColumnExists(context.Background(), tx, `api_keys;DROP TABLE users`, "user_id")
	require.ErrorContains(t, err, "unsafe")
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserScopedDataEvidenceRejectsTamperedCountsAndChecksums(t *testing.T) {
	policy := []userScopedDataPolicyEntry{{
		public:   UserScopedDataPolicyEntry{Table: "api_keys", Column: "user_id", Disposition: UserDataMigrateToOperator},
		identity: userDataIdentityID,
	}}
	valid := UserScopedDataEvidence{
		Table: "api_keys", Column: "user_id", Disposition: UserDataMigrateToOperator, Status: "applied",
		BeforeRows: 2, AfterRows: 2, ChangedRows: 2,
		BeforeIDChecksum: strings.Repeat("a", 32), AfterIDChecksum: strings.Repeat("a", 32),
	}
	require.NoError(t, validateUserScopedDataEvidenceWithPolicy([]UserScopedDataEvidence{valid}, policy))
	valid.AfterIDChecksum = strings.Repeat("b", 32)
	require.ErrorContains(t, validateUserScopedDataEvidenceWithPolicy([]UserScopedDataEvidence{valid}, policy), "identity set changed")
}
