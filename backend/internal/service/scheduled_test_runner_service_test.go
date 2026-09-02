package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type claimingScheduledPlanRepo struct {
	mu             sync.Mutex
	plan           ScheduledTestPlan
	claimCalls     int
	completeCalls  int
	completeResult bool
}

func (r *claimingScheduledPlanRepo) Create(context.Context, *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *claimingScheduledPlanRepo) GetByID(context.Context, int64) (*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *claimingScheduledPlanRepo) ListByAccountID(context.Context, int64) ([]*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *claimingScheduledPlanRepo) ListDue(context.Context, time.Time) ([]*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *claimingScheduledPlanRepo) Update(context.Context, *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *claimingScheduledPlanRepo) Delete(context.Context, int64) error { return nil }
func (r *claimingScheduledPlanRepo) UpdateAfterRun(context.Context, int64, time.Time, time.Time) error {
	return nil
}

func (r *claimingScheduledPlanRepo) ClaimDue(_ context.Context, now, leaseUntil time.Time, _ int) ([]*ScheduledTestPlan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimCalls++
	if !r.plan.Enabled || r.plan.NextRunAt == nil || r.plan.NextRunAt.After(now) {
		return nil, nil
	}
	r.plan.NextRunAt = &leaseUntil
	claimed := r.plan
	return []*ScheduledTestPlan{&claimed}, nil
}

func (r *claimingScheduledPlanRepo) CompleteClaimedRun(_ context.Context, id int64, leaseUntil, lastRunAt, nextRunAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completeCalls++
	if r.plan.ID != id || r.plan.NextRunAt == nil || !r.plan.NextRunAt.Equal(leaseUntil) {
		return false, nil
	}
	if !r.completeResult {
		return false, nil
	}
	r.plan.LastRunAt = &lastRunAt
	r.plan.NextRunAt = &nextRunAt
	return true, nil
}

type scheduledRunnerAccountStub struct {
	mu     sync.Mutex
	calls  int
	result *ScheduledTestResult
	err    error
}

func (s *scheduledRunnerAccountStub) RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.result, s.err
}

type scheduledRunnerResultStub struct {
	mu    sync.Mutex
	calls int
}

func (s *scheduledRunnerResultStub) SaveResult(context.Context, int64, int, *ScheduledTestResult) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return nil
}

func TestScheduledTestRunnerClaimsOnlyOnceAcrossConcurrentCycles(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	repo := &claimingScheduledPlanRepo{plan: ScheduledTestPlan{
		ID: 1, AccountID: 42, CronExpression: "0 0 1 1 *", Enabled: true,
		MaxResults: 50, NextRunAt: func() *time.Time { value := now.Add(-time.Minute); return &value }(),
	}, completeResult: true}
	account := &scheduledRunnerAccountStub{result: &ScheduledTestResult{Status: "success"}}
	results := &scheduledRunnerResultStub{}
	runner := NewScheduledTestRunnerService(repo, results, account, nil, nil)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runner.runScheduledAt(context.Background(), now)
		}()
	}
	wg.Wait()

	repo.mu.Lock()
	claimCalls, completeCalls := repo.claimCalls, repo.completeCalls
	repo.mu.Unlock()
	account.mu.Lock()
	accountCalls := account.calls
	account.mu.Unlock()
	results.mu.Lock()
	resultCalls := results.calls
	results.mu.Unlock()

	require.Equal(t, 2, claimCalls)
	require.Equal(t, 1, accountCalls, "only the lease owner may call the upstream test")
	require.Equal(t, 1, resultCalls)
	require.Equal(t, 1, completeCalls)
}

func TestScheduledTestRunnerRetainsLeaseAfterTestFailure(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	repo := &claimingScheduledPlanRepo{plan: ScheduledTestPlan{
		ID: 1, AccountID: 42, CronExpression: "0 0 1 1 *", Enabled: true,
		MaxResults: 50, NextRunAt: func() *time.Time { value := now.Add(-time.Minute); return &value }(),
	}, completeResult: true}
	account := &scheduledRunnerAccountStub{err: errors.New("upstream unavailable")}
	results := &scheduledRunnerResultStub{}
	runner := NewScheduledTestRunnerService(repo, results, account, nil, nil)

	runner.runScheduledAt(context.Background(), now)

	repo.mu.Lock()
	leaseUntil := *repo.plan.NextRunAt
	completeCalls := repo.completeCalls
	repo.mu.Unlock()
	require.Equal(t, now.Add(scheduledTestClaimLease), leaseUntil)
	require.Zero(t, completeCalls, "failed tests must leave the lease for expiry instead of completing it")

	_, err := repo.ClaimDue(context.Background(), now.Add(time.Minute), now.Add(11*time.Minute), 10)
	require.NoError(t, err)
	repo.mu.Lock()
	claimCalls := repo.claimCalls
	repo.mu.Unlock()
	require.Equal(t, 2, claimCalls)
	claimed, err := repo.ClaimDue(context.Background(), now.Add(scheduledTestClaimLease+time.Second), now.Add(20*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "a failed run becomes retryable after the lease expires")
}
