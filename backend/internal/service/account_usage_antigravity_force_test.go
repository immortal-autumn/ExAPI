package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type countingAntigravityUsageFetcher struct {
	mu    sync.Mutex
	calls int
}

func (f *countingAntigravityUsageFetcher) CanFetch(*Account) bool { return true }

func (f *countingAntigravityUsageFetcher) GetProxyURL(context.Context, *Account) string {
	return ""
}

func (f *countingAntigravityUsageFetcher) FetchQuota(context.Context, *Account, string) (*QuotaResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	updatedAt := time.Date(2026, 8, 19, 12, 0, f.calls, 0, time.UTC)
	return &QuotaResult{UsageInfo: &UsageInfo{
		Source:    fmt.Sprintf("fetch-%d", f.calls),
		UpdatedAt: &updatedAt,
	}}, nil
}

func (f *countingAntigravityUsageFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestGetAntigravityUsageForceBypassesFreshCache(t *testing.T) {
	t.Parallel()

	fetcher := &countingAntigravityUsageFetcher{}
	svc := &AccountUsageService{
		antigravityQuotaFetcher: fetcher,
		cache:                   NewUsageCache(),
	}
	account := &Account{ID: 22, Platform: PlatformAntigravity}

	first, err := svc.getAntigravityUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("initial getAntigravityUsage() error = %v", err)
	}
	cached, err := svc.getAntigravityUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("cached getAntigravityUsage() error = %v", err)
	}
	forced, err := svc.getAntigravityUsage(context.Background(), account, true)
	if err != nil {
		t.Fatalf("forced getAntigravityUsage() error = %v", err)
	}

	if got := fetcher.callCount(); got != 2 {
		t.Fatalf("FetchQuota() calls = %d, want 2 (initial + forced)", got)
	}
	if first.Source != "fetch-1" || cached.Source != "fetch-1" {
		t.Fatalf("ordinary cache was not reused: first=%q cached=%q", first.Source, cached.Source)
	}
	if forced.Source != "fetch-2" {
		t.Fatalf("forced refresh returned %q, want fetch-2", forced.Source)
	}
}
