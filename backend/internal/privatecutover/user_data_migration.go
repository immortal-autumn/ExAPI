package privatecutover

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// UserScopedDataPolicySchemaVersion identifies the user-reference matrix
// encoded in MigrationReport. A zero value remains valid for reports written
// by the older v2 cutover implementation.
const UserScopedDataPolicySchemaVersion = 1

// UserDataDisposition is deliberately finite. New user references must be
// assigned one of these outcomes before a private cutover can be released.
type UserDataDisposition string

const (
	UserDataMigrateToOperator        UserDataDisposition = "migrate_to_operator"
	UserDataNullHistoricalRef        UserDataDisposition = "null_historical_reference"
	UserDataDeleteCustomerRows       UserDataDisposition = "delete_customer_rows"
	UserDataRetainHistoricalSnapshot UserDataDisposition = "retain_historical_snapshot"
)

// UserScopedDataEvidence is signed as part of the cutover report. Counts and
// identity checksums cover complete rows in each present table; no payload,
// credential, or customer content is copied into the report.
type UserScopedDataEvidence struct {
	Table            string              `json:"table"`
	Column           string              `json:"column"`
	Disposition      UserDataDisposition `json:"disposition"`
	Status           string              `json:"status"`
	BeforeRows       int64               `json:"before_rows"`
	AfterRows        int64               `json:"after_rows"`
	ChangedRows      int64               `json:"changed_rows"`
	BeforeIDChecksum string              `json:"before_id_checksum,omitempty"`
	AfterIDChecksum  string              `json:"after_id_checksum,omitempty"`
}

// UserScopedDataPolicyEntry is the reviewable, public representation of the
// matrix. SQL identity/scope expressions are kept private and derived only
// from these compile-time table/column pairs.
type UserScopedDataPolicyEntry struct {
	Table       string              `json:"table"`
	Column      string              `json:"column"`
	Disposition UserDataDisposition `json:"disposition"`
}

// UserScopedDataPolicy is the single source of truth for every user-scoped
// table retained by the current schema. Missing tables/columns are recorded as
// skipped because older installations may not have run every optional
// migration yet; no arbitrary identifier can enter the SQL path.
var UserScopedDataPolicy = []UserScopedDataPolicyEntry{
	{Table: "api_keys", Column: "user_id", Disposition: UserDataMigrateToOperator},
	{Table: "usage_logs", Column: "user_id", Disposition: UserDataMigrateToOperator},
	{Table: "billing_usage_entries", Column: "user_id", Disposition: UserDataMigrateToOperator},
	{Table: "usage_cleanup_tasks", Column: "created_by", Disposition: UserDataMigrateToOperator},
	{Table: "audit_logs", Column: "actor_user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_system_logs", Column: "user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_error_logs", Column: "user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_error_logs", Column: "resolved_by_user_id", Disposition: UserDataNullHistoricalRef},
	// deleted_key_owner_user_id is a historical owner snapshot attached to an
	// operational error. It must not keep a deleted customer's identifier
	// addressable after the cutover.
	{Table: "ops_error_logs", Column: "deleted_key_owner_user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_retry_attempts", Column: "requested_by_user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_alert_silences", Column: "created_by", Disposition: UserDataNullHistoricalRef},
	{Table: "content_moderation_logs", Column: "user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "prompt_audit_jobs", Column: "user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "prompt_audit_events", Column: "user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "user_allowed_groups", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_group_rate_multipliers", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_provider_default_grants", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_avatars", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "sora_generations", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "usage_dashboard_hourly_users", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "usage_dashboard_daily_users", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "orphan_allowed_groups_audit", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "ops_ingress_reject_aggregates", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "deleted_api_key_audits", Column: "user_id", Disposition: UserDataRetainHistoricalSnapshot},
	{Table: "auth_identity_migration_reports", Column: "details", Disposition: UserDataRetainHistoricalSnapshot},
	{Table: "channel_monitors", Column: "created_by", Disposition: UserDataMigrateToOperator},
	{Table: "auth_identity_migration_reports", Column: "resolved_by_user_id", Disposition: UserDataNullHistoricalRef},
	{Table: "user_subscriptions", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_subscriptions", Column: "assigned_by", Disposition: UserDataDeleteCustomerRows},
	{Table: "redeem_codes", Column: "used_by", Disposition: UserDataDeleteCustomerRows},
	{Table: "promo_code_usages", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "announcements", Column: "created_by", Disposition: UserDataDeleteCustomerRows},
	{Table: "announcement_reads", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_attribute_values", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_platform_quotas", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "auth_identities", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "pending_auth_sessions", Column: "target_user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_affiliates", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_affiliates", Column: "inviter_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_affiliate_ledger", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "user_affiliate_ledger", Column: "source_user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "passkey_user_handles", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "passkey_credentials", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	{Table: "payment_orders", Column: "user_id", Disposition: UserDataDeleteCustomerRows},
	// These references are defined by optional/late migrations and are kept at
	// the end of the matrix so adding them cannot renumber the established
	// evidence order above.
	{Table: "usage_cleanup_tasks", Column: "canceled_by", Disposition: UserDataNullHistoricalRef},
	{Table: "announcements", Column: "updated_by", Disposition: UserDataNullHistoricalRef},
	{Table: "ops_system_log_cleanup_audits", Column: "operator_id", Disposition: UserDataMigrateToOperator},
}

type userDataIdentityKind uint8

const (
	userDataIdentityID userDataIdentityKind = iota + 1
	userDataIdentityUserGroup
	userDataIdentityHourlyUser
	userDataIdentityDailyUser
)

type userScopedDataPolicyEntry struct {
	public     UserScopedDataPolicyEntry
	identity   userDataIdentityKind
	keepGlobal bool
}

var userScopedDataPolicy = []userScopedDataPolicyEntry{
	{public: UserScopedDataPolicy[0], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[1], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[2], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[3], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[4], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[5], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[6], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[7], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[8], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[9], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[10], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[11], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[12], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[13], identity: userDataIdentityUserGroup},
	{public: UserScopedDataPolicy[14], identity: userDataIdentityUserGroup},
	{public: UserScopedDataPolicy[15], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[16], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[17], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[18], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[19], identity: userDataIdentityHourlyUser},
	{public: UserScopedDataPolicy[20], identity: userDataIdentityDailyUser},
	{public: UserScopedDataPolicy[21], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[22], identity: userDataIdentityID, keepGlobal: true},
	{public: UserScopedDataPolicy[23], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[24], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[25], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[26], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[27], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[28], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[29], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[30], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[31], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[32], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[33], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[34], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[35], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[36], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[37], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[38], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[39], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[40], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[41], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[42], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[43], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[44], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[45], identity: userDataIdentityID},
	{public: UserScopedDataPolicy[46], identity: userDataIdentityID},
}

var safeSQLIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func validateUserScopedDataPolicy() error {
	if len(UserScopedDataPolicy) != len(userScopedDataPolicy) {
		return errors.New("user-scoped data policy public/private matrix length mismatch")
	}
	seen := make(map[string]struct{}, len(userScopedDataPolicy))
	for index, entry := range userScopedDataPolicy {
		if entry.public != UserScopedDataPolicy[index] {
			return fmt.Errorf("user-scoped data policy entry %d diverges from public matrix", index)
		}
		if !safeSQLIdentifier.MatchString(entry.public.Table) || !safeSQLIdentifier.MatchString(entry.public.Column) {
			return fmt.Errorf("user-scoped data policy contains unsafe identifier %s.%s", entry.public.Table, entry.public.Column)
		}
		key := entry.public.Table + "\x00" + entry.public.Column
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate user-scoped data policy entry %s.%s", entry.public.Table, entry.public.Column)
		}
		seen[key] = struct{}{}
		if err := validateUserScopedDataPolicyEntryShallow(entry); err != nil {
			return err
		}
	}
	return nil
}

func validateUserScopedDataEvidence(evidence []UserScopedDataEvidence) error {
	return validateUserScopedDataEvidenceWithPolicy(evidence, userScopedDataPolicy)
}

func validateUserScopedDataEvidenceWithPolicy(evidence []UserScopedDataEvidence, policy []userScopedDataPolicyEntry) error {
	if len(evidence) == 0 {
		// Compatibility with reports generated before this matrix was added.
		return nil
	}
	if err := validateUserScopedDataPolicyEntries(policy); err != nil {
		return err
	}
	if len(evidence) != len(policy) {
		return fmt.Errorf("expected %d user-scoped data evidence entries, got %d", len(policy), len(evidence))
	}
	for index, item := range evidence {
		policyEntry := policy[index].public
		if item.Table != policyEntry.Table || item.Column != policyEntry.Column || item.Disposition != policyEntry.Disposition {
			return fmt.Errorf("user-scoped data evidence entry %d does not match policy", index)
		}
		switch item.Status {
		case "applied":
			if item.BeforeRows < 0 || item.AfterRows < 0 || item.ChangedRows < 0 || item.ChangedRows > item.BeforeRows {
				return fmt.Errorf("invalid row counts for user-scoped data evidence %s.%s", item.Table, item.Column)
			}
			if err := validateIdentityChecksum(item.BeforeIDChecksum); err != nil {
				return fmt.Errorf("invalid before identity checksum for %s.%s: %w", item.Table, item.Column, err)
			}
			if err := validateIdentityChecksum(item.AfterIDChecksum); err != nil {
				return fmt.Errorf("invalid after identity checksum for %s.%s: %w", item.Table, item.Column, err)
			}
			if (item.Disposition == UserDataMigrateToOperator || item.Disposition == UserDataNullHistoricalRef || item.Disposition == UserDataRetainHistoricalSnapshot) &&
				(item.BeforeRows != item.AfterRows || item.BeforeIDChecksum != item.AfterIDChecksum) {
				return fmt.Errorf("identity set changed for normalized user-scoped data evidence %s.%s", item.Table, item.Column)
			}
			if item.Disposition == UserDataDeleteCustomerRows && item.AfterRows > item.BeforeRows {
				return fmt.Errorf("row count increased for deleted user-scoped data evidence %s.%s", item.Table, item.Column)
			}
		case "skipped_missing_table", "skipped_missing_column":
			if item.BeforeRows != 0 || item.AfterRows != 0 || item.ChangedRows != 0 || item.BeforeIDChecksum != "" || item.AfterIDChecksum != "" {
				return fmt.Errorf("skipped user-scoped data evidence %s.%s contains data", item.Table, item.Column)
			}
		default:
			return fmt.Errorf("unsupported user-scoped data evidence status %q", item.Status)
		}
	}
	return nil
}

func validateUserScopedDataPolicyEntries(policy []userScopedDataPolicyEntry) error {
	seen := make(map[string]struct{}, len(policy))
	for _, entry := range policy {
		if err := validateUserScopedDataPolicyEntryShallow(entry); err != nil {
			return err
		}
		key := entry.public.Table + "\x00" + entry.public.Column
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate user-scoped data policy entry %s.%s", entry.public.Table, entry.public.Column)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateIdentityChecksum(value string) error {
	if len(value) != 32 || value != strings.ToLower(value) {
		return errors.New("checksum must be a lowercase MD5 digest")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return err
	}
	return nil
}

type userScopedTableSnapshot struct {
	rows     int64
	checksum string
}

func migrateUserScopedData(ctx context.Context, tx *sql.Tx, operatorID int64) ([]UserScopedDataEvidence, error) {
	// Keep the exported review matrix and the private SQL metadata locked
	// together. The helper below intentionally accepts a custom policy for
	// isolated tests, but production cutover must always validate the complete
	// compile-time matrix first.
	if err := validateUserScopedDataPolicy(); err != nil {
		return nil, err
	}
	return migrateUserScopedDataWithPolicy(ctx, tx, operatorID, userScopedDataPolicy)
}

func migrateUserScopedDataWithPolicy(ctx context.Context, tx *sql.Tx, operatorID int64, policy []userScopedDataPolicyEntry) ([]UserScopedDataEvidence, error) {
	if operatorID <= 0 {
		return nil, errors.New("operator id must be positive")
	}
	if err := validateUserScopedDataPolicyEntries(policy); err != nil {
		return nil, err
	}
	evidence := make([]UserScopedDataEvidence, 0, len(policy))
	for _, entry := range policy {
		if err := validateUserScopedDataPolicyEntry(entry); err != nil {
			return nil, err
		}
		exists, err := userScopedTableColumnExists(ctx, tx, entry.public.Table, entry.public.Column)
		if err != nil {
			return nil, fmt.Errorf("inspect user-scoped data %s.%s: %w", entry.public.Table, entry.public.Column, err)
		}
		if !exists.table {
			evidence = append(evidence, skippedUserScopedEvidence(entry.public, "skipped_missing_table"))
			continue
		}
		if !exists.column {
			evidence = append(evidence, skippedUserScopedEvidence(entry.public, "skipped_missing_column"))
			continue
		}
		if entry.public.Disposition == UserDataNullHistoricalRef && !exists.nullable {
			return nil, fmt.Errorf("user-scoped historical reference %s.%s is not nullable; refusing to leave a deleted user id", entry.public.Table, entry.public.Column)
		}
		before, err := snapshotUserScopedTable(ctx, tx, entry)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s.%s before cutover: %w", entry.public.Table, entry.public.Column, err)
		}
		changedRows, err := applyUserScopedDataDisposition(ctx, tx, operatorID, entry)
		if err != nil {
			return nil, fmt.Errorf("apply %s to %s.%s: %w", entry.public.Disposition, entry.public.Table, entry.public.Column, err)
		}
		after, err := snapshotUserScopedTable(ctx, tx, entry)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s.%s after cutover: %w", entry.public.Table, entry.public.Column, err)
		}
		item := UserScopedDataEvidence{
			Table:            entry.public.Table,
			Column:           entry.public.Column,
			Disposition:      entry.public.Disposition,
			Status:           "applied",
			BeforeRows:       before.rows,
			AfterRows:        after.rows,
			ChangedRows:      changedRows,
			BeforeIDChecksum: before.checksum,
			AfterIDChecksum:  after.checksum,
		}
		if (entry.public.Disposition == UserDataMigrateToOperator || entry.public.Disposition == UserDataNullHistoricalRef) &&
			(before.rows != after.rows || before.checksum != after.checksum) {
			return nil, fmt.Errorf("identity set changed while normalizing %s.%s", entry.public.Table, entry.public.Column)
		}
		if entry.public.Disposition == UserDataRetainHistoricalSnapshot &&
			(before.rows != after.rows || before.checksum != after.checksum) {
			return nil, fmt.Errorf("historical snapshot %s.%s changed unexpectedly", entry.public.Table, entry.public.Column)
		}
		if entry.public.Disposition == UserDataDeleteCustomerRows && after.rows > before.rows {
			return nil, fmt.Errorf("customer-row cleanup increased %s.%s row count", entry.public.Table, entry.public.Column)
		}
		evidence = append(evidence, item)
	}
	if err := validateUserScopedDataEvidenceWithPolicy(evidence, policy); err != nil {
		return nil, err
	}
	return evidence, nil
}

type userScopedTableColumnPresence struct {
	table    bool
	column   bool
	nullable bool
}

func userScopedTableColumnExists(ctx context.Context, tx *sql.Tx, table, column string) (userScopedTableColumnPresence, error) {
	if !safeSQLIdentifier.MatchString(table) || !safeSQLIdentifier.MatchString(column) {
		return userScopedTableColumnPresence{}, errors.New("unsafe user-scoped table or column identifier")
	}
	var tableExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = $1
		  AND c.relkind IN ('r', 'p')
	)`, table).Scan(&tableExists); err != nil {
		return userScopedTableColumnPresence{}, err
	}
	if !tableExists {
		return userScopedTableColumnPresence{}, nil
	}
	var columnExists bool
	var nullable string
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
	), COALESCE((SELECT is_nullable FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2 LIMIT 1), 'NO')`, table, column).Scan(&columnExists, &nullable); err != nil {
		return userScopedTableColumnPresence{}, err
	}
	return userScopedTableColumnPresence{table: true, column: columnExists, nullable: columnExists && nullable == "YES"}, nil
}

func skippedUserScopedEvidence(policy UserScopedDataPolicyEntry, status string) UserScopedDataEvidence {
	return UserScopedDataEvidence{Table: policy.Table, Column: policy.Column, Disposition: policy.Disposition, Status: status}
}

func validateUserScopedDataPolicyEntry(entry userScopedDataPolicyEntry) error {
	return validateUserScopedDataPolicyEntryShallow(entry)
}

func snapshotUserScopedTable(ctx context.Context, tx *sql.Tx, entry userScopedDataPolicyEntry) (userScopedTableSnapshot, error) {
	identity, err := userScopedIdentitySQL(entry)
	if err != nil {
		return userScopedTableSnapshot{}, err
	}
	table := quoteIdentifier(entry.public.Table)
	// Explicitly include NULL identities in the checksum. Without the
	// sentinel, PostgreSQL string_agg ignores NULL values, so a nullable
	// composite identity could change while its checksum appeared unchanged.
	query := fmt.Sprintf(`SELECT COUNT(*), COALESCE(md5(string_agg(COALESCE((%s)::text, '<NULL>'), ',' ORDER BY (%s))), md5('')) FROM %s`, identity, identity, table)
	var snapshot userScopedTableSnapshot
	if err := tx.QueryRowContext(ctx, query).Scan(&snapshot.rows, &snapshot.checksum); err != nil {
		return userScopedTableSnapshot{}, err
	}
	if err := validateIdentityChecksum(snapshot.checksum); err != nil {
		return userScopedTableSnapshot{}, err
	}
	return snapshot, nil
}

func userScopedIdentitySQL(entry userScopedDataPolicyEntry) (string, error) {
	if err := validateUserScopedDataPolicyEntryShallow(entry); err != nil {
		return "", err
	}
	user := quoteIdentifier(entry.public.Column)
	switch entry.identity {
	case userDataIdentityID:
		return quoteIdentifier("id"), nil
	case userDataIdentityUserGroup:
		return user + `::text || ':' || "group_id"::text`, nil
	case userDataIdentityHourlyUser:
		return `"bucket_start"::text || ':' || "user_id"::text`, nil
	case userDataIdentityDailyUser:
		return `"bucket_date"::text || ':' || "user_id"::text`, nil
	default:
		return "", errors.New("unsupported user-scoped identity expression")
	}
}

func validateUserScopedDataPolicyEntryShallow(entry userScopedDataPolicyEntry) error {
	if !safeSQLIdentifier.MatchString(entry.public.Table) || !safeSQLIdentifier.MatchString(entry.public.Column) {
		return errors.New("unsafe user-scoped table or column identifier")
	}
	switch entry.public.Disposition {
	case UserDataMigrateToOperator, UserDataNullHistoricalRef, UserDataDeleteCustomerRows, UserDataRetainHistoricalSnapshot:
	default:
		return fmt.Errorf("unsupported user-scoped disposition %q", entry.public.Disposition)
	}
	return nil
}

func applyUserScopedDataDisposition(ctx context.Context, tx *sql.Tx, operatorID int64, entry userScopedDataPolicyEntry) (int64, error) {
	if err := validateUserScopedDataPolicyEntryShallow(entry); err != nil {
		return 0, err
	}
	if entry.public.Disposition == UserDataRetainHistoricalSnapshot {
		return 0, nil
	}
	column := quoteIdentifier(entry.public.Column)
	where := fmt.Sprintf("%s IS NOT NULL AND %s <> $1", column, column)
	if entry.keepGlobal {
		where = fmt.Sprintf("%s <> 0 AND %s <> $1", column, column)
	}
	table := quoteIdentifier(entry.public.Table)
	var query string
	switch entry.public.Disposition {
	case UserDataMigrateToOperator:
		query = fmt.Sprintf("UPDATE %s SET %s = $1 WHERE %s", table, column, where)
	case UserDataNullHistoricalRef:
		query = fmt.Sprintf("UPDATE %s SET %s = NULL WHERE %s", table, column, where)
	case UserDataDeleteCustomerRows:
		query = fmt.Sprintf("DELETE FROM %s WHERE %s", table, where)
	default:
		return 0, fmt.Errorf("unsupported user-scoped disposition %q", entry.public.Disposition)
	}
	result, err := tx.ExecContext(ctx, query, operatorID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
