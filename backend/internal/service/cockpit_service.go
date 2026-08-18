package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const cockpitWarningLimit = 6

type CockpitAccountCounts struct {
	Total             int64 `json:"total"`
	Active            int64 `json:"active"`
	Inactive          int64 `json:"inactive"`
	Error             int64 `json:"error"`
	DispatchEligible  int64 `json:"dispatch_eligible"`
	QuotaWarningTotal int64 `json:"quota_warning_total"`
}

type CockpitPlatformCounts struct {
	Platform         string `json:"platform"`
	Total            int64  `json:"total"`
	Active           int64  `json:"active"`
	Error            int64  `json:"error"`
	DispatchEligible int64  `json:"dispatch_eligible"`
}

type CockpitQuotaWarning struct {
	AccountID int64   `json:"account_id"`
	Name      string  `json:"name"`
	Platform  string  `json:"platform"`
	Scope     string  `json:"scope"`
	Used      float64 `json:"used"`
	Limit     float64 `json:"limit"`
	Percent   float64 `json:"percent"`
	Severity  string  `json:"severity"`
}

type CockpitSummary struct {
	GeneratedAt   time.Time               `json:"generated_at"`
	Accounts      CockpitAccountCounts    `json:"accounts"`
	Platforms     []CockpitPlatformCounts `json:"platforms"`
	QuotaWarnings []CockpitQuotaWarning   `json:"quota_warnings"`
}

type CockpitService struct{ db *sql.DB }

func NewCockpitService(db *sql.DB) *CockpitService { return &CockpitService{db: db} }

const cockpitNumeric = `CASE WHEN jsonb_typeof(extra->'%s') = 'number' THEN (extra->>'%s')::numeric ELSE 0 END`

const (
	cockpitQuotaAccountTypes = `type IN ('apikey','bedrock')`
	cockpitDailyWindowActive = `(CASE
		WHEN COALESCE(extra->>'quota_daily_reset_mode', 'rolling') = 'fixed'
		THEN COALESCE(NULLIF(extra->>'quota_daily_reset_at', '')::timestamptz, '1970-01-01'::timestamptz) > NOW()
		ELSE COALESCE(NULLIF(extra->>'quota_daily_start', '')::timestamptz, '1970-01-01'::timestamptz) + '24 hours'::interval > NOW()
	END)`
	cockpitWeeklyWindowActive = `(CASE
		WHEN COALESCE(extra->>'quota_weekly_reset_mode', 'rolling') = 'fixed'
		THEN COALESCE(NULLIF(extra->>'quota_weekly_reset_at', '')::timestamptz, '1970-01-01'::timestamptz) > NOW()
		ELSE COALESCE(NULLIF(extra->>'quota_weekly_start', '')::timestamptz, '1970-01-01'::timestamptz) + '168 hours'::interval > NOW()
	END)`
)

func (s *CockpitService) GetSummary(ctx context.Context) (*CockpitSummary, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("cockpit database is unavailable")
	}
	rows, err := s.db.QueryContext(ctx, cockpitCountsQuery())
	if err != nil {
		return nil, fmt.Errorf("query cockpit counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	summary := &CockpitSummary{GeneratedAt: time.Now().UTC(), Platforms: []CockpitPlatformCounts{}, QuotaWarnings: []CockpitQuotaWarning{}}
	for rows.Next() {
		var item CockpitPlatformCounts
		var inactive int64
		if err := rows.Scan(&item.Platform, &item.Total, &item.Active, &inactive, &item.Error, &item.DispatchEligible); err != nil {
			return nil, err
		}
		summary.Platforms = append(summary.Platforms, item)
		summary.Accounts.Total += item.Total
		summary.Accounts.Active += item.Active
		summary.Accounts.Inactive += inactive
		summary.Accounts.Error += item.Error
		summary.Accounts.DispatchEligible += item.DispatchEligible
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	warnings, total, err := s.quotaWarnings(ctx)
	if err != nil {
		return nil, err
	}
	summary.QuotaWarnings = warnings
	summary.Accounts.QuotaWarningTotal = total
	return summary, nil
}

func cockpitCountsQuery() string {
	totalLimit := fmt.Sprintf(cockpitNumeric, "quota_limit", "quota_limit")
	totalUsed := fmt.Sprintf(cockpitNumeric, "quota_used", "quota_used")
	dailyLimit := fmt.Sprintf(cockpitNumeric, "quota_daily_limit", "quota_daily_limit")
	dailyUsed := fmt.Sprintf(cockpitNumeric, "quota_daily_used", "quota_daily_used")
	weeklyLimit := fmt.Sprintf(cockpitNumeric, "quota_weekly_limit", "quota_weekly_limit")
	weeklyUsed := fmt.Sprintf(cockpitNumeric, "quota_weekly_used", "quota_weekly_used")
	quotaExceeded := fmt.Sprintf(`%s AND (
		(%s > 0 AND %s >= %s)
		OR (%s AND %s > 0 AND %s >= %s)
		OR (%s AND %s > 0 AND %s >= %s)
	)`, cockpitQuotaAccountTypes,
		totalLimit, totalUsed, totalLimit,
		cockpitDailyWindowActive, dailyLimit, dailyUsed, dailyLimit,
		cockpitWeeklyWindowActive, weeklyLimit, weeklyUsed, weeklyLimit,
	)
	return fmt.Sprintf(`
		SELECT platform,
			COUNT(*)::bigint,
			COUNT(*) FILTER (WHERE status='active')::bigint,
			COUNT(*) FILTER (WHERE status NOT IN ('active','error'))::bigint,
			COUNT(*) FILTER (WHERE status='error')::bigint,
			COUNT(*) FILTER (WHERE
				status='active' AND schedulable
				AND (NOT auto_pause_on_expired OR expires_at IS NULL OR expires_at > NOW())
				AND (rate_limit_reset_at IS NULL OR rate_limit_reset_at <= NOW())
				AND (overload_until IS NULL OR overload_until <= NOW())
				AND (temp_unschedulable_until IS NULL OR temp_unschedulable_until <= NOW())
				AND NOT (%s)
			)::bigint
		FROM accounts
		WHERE deleted_at IS NULL
		GROUP BY platform
		ORDER BY platform`, quotaExceeded)
}

func (s *CockpitService) quotaWarnings(ctx context.Context) ([]CockpitQuotaWarning, int64, error) {
	rows, err := s.db.QueryContext(ctx, cockpitWarningsQuery(), cockpitWarningLimit)
	if err != nil {
		return nil, 0, fmt.Errorf("query cockpit quota warnings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	warnings := make([]CockpitQuotaWarning, 0, cockpitWarningLimit)
	var total int64
	for rows.Next() {
		var item CockpitQuotaWarning
		if err := rows.Scan(&item.AccountID, &item.Name, &item.Platform, &item.Scope, &item.Used, &item.Limit, &item.Percent, &item.Severity, &total); err != nil {
			return nil, 0, err
		}
		warnings = append(warnings, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return warnings, total, nil
}

func cockpitWarningsQuery() string {
	totalUsed := fmt.Sprintf(cockpitNumeric, "quota_used", "quota_used")
	totalLimit := fmt.Sprintf(cockpitNumeric, "quota_limit", "quota_limit")
	dailyUsed := fmt.Sprintf(cockpitNumeric, "quota_daily_used", "quota_daily_used")
	dailyLimit := fmt.Sprintf(cockpitNumeric, "quota_daily_limit", "quota_daily_limit")
	weeklyUsed := fmt.Sprintf(cockpitNumeric, "quota_weekly_used", "quota_weekly_used")
	weeklyLimit := fmt.Sprintf(cockpitNumeric, "quota_weekly_limit", "quota_weekly_limit")
	return fmt.Sprintf(`
		WITH windows AS (
			SELECT id, name, platform, 'total'::text AS scope, %s AS used, %s AS quota_limit
			FROM accounts WHERE deleted_at IS NULL AND %s
			UNION ALL
			SELECT id, name, platform, 'daily'::text AS scope, %s AS used, %s AS quota_limit
			FROM accounts WHERE deleted_at IS NULL AND %s AND %s
			UNION ALL
			SELECT id, name, platform, 'weekly'::text AS scope, %s AS used, %s AS quota_limit
			FROM accounts WHERE deleted_at IS NULL AND %s AND %s
		), ranked AS (
			SELECT *, ROUND((used / NULLIF(quota_limit, 0)) * 100, 2) AS percent,
				ROW_NUMBER() OVER (PARTITION BY id ORDER BY (used / NULLIF(quota_limit, 0)) DESC NULLS LAST, scope) AS rn
			FROM windows WHERE quota_limit > 0 AND (used / quota_limit) >= 0.70
		), worst AS (
			SELECT * FROM ranked WHERE rn=1
		)
		SELECT id, name, platform, scope, used::float8, quota_limit::float8, percent::float8,
			CASE WHEN percent >= 90 THEN 'critical' ELSE 'warning' END,
			COUNT(*) OVER ()::bigint
		FROM worst
		ORDER BY percent DESC, id ASC
		LIMIT $1`,
		totalUsed, totalLimit, cockpitQuotaAccountTypes,
		dailyUsed, dailyLimit, cockpitQuotaAccountTypes, cockpitDailyWindowActive,
		weeklyUsed, weeklyLimit, cockpitQuotaAccountTypes, cockpitWeeklyWindowActive,
	)
}
