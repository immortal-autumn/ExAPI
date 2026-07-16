package service

import (
	"context"
	"testing"
	"time"
)

type denyWorkerLeaderLock struct{}

func (denyWorkerLeaderLock) TryAcquireLeaderLock(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}

func (denyWorkerLeaderLock) ReleaseLeaderLock(context.Context, string, string) error {
	return nil
}

func TestProvideEmailQueueServiceUsesOperatorMinimumInPrivateMode(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")

	svc := ProvideEmailQueueService(&EmailService{})
	t.Cleanup(svc.Stop)

	if svc.workers != 1 {
		t.Fatalf("private email queue workers = %d, want 1 for operator TOTP", svc.workers)
	}
}

func TestProvideEmailQueueServiceKeepsDefaultOutsidePrivateMode(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "false")

	svc := ProvideEmailQueueService(&EmailService{})
	t.Cleanup(svc.Stop)

	if svc.workers != 3 {
		t.Fatalf("standard email queue workers = %d, want 3", svc.workers)
	}
}

func TestProvidePaymentOrderExpiryServiceDoesNotStartInPrivateMode(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")

	svc := ProvidePaymentOrderExpiryService(&PaymentService{}, denyWorkerLeaderLock{}, nil)
	t.Cleanup(svc.Stop)

	if svc.Running() {
		t.Fatal("payment order expiry worker is running in private mode")
	}
}

func TestProvidePaymentOrderExpiryServiceStartsOutsidePrivateMode(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "false")

	svc := ProvidePaymentOrderExpiryService(&PaymentService{}, denyWorkerLeaderLock{}, nil)
	t.Cleanup(svc.Stop)

	if !svc.Running() {
		t.Fatal("payment order expiry worker is not running outside private mode")
	}
}
