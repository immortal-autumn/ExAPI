//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

// TestTotpLogsNeverIncludeSecret exercises both setup and verification with a
// known sentinel secret. The secret is valid Base32 so the real TOTP code path
// runs; a regression that logs a prefix or decrypted value is caught by the
// captured log assertion below.
func TestTotpLogsNeverIncludeSecret(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"

	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	user := &User{ID: 7, TotpEnabled: false}
	repo := &totpLoggingUserRepo{user: user}
	cache := &totpLoggingCache{session: &TotpSetupSession{Secret: secret, SetupToken: "setup-token"}}
	encryptor := &totpLoggingEncryptor{secret: secret}
	settings := NewSettingService(&totpLoggingSettingRepo{values: map[string]string{
		SettingKeyTotpEnabled: "true",
	}}, nil)
	service := NewTotpService(repo, encryptor, cache, settings, nil, nil)

	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, service.CompleteSetup(context.Background(), user.ID, code, "setup-token"))
	require.NoError(t, service.VerifyCode(context.Background(), user.ID, code))

	logs := output.String()
	require.NotContains(t, logs, secret)
	require.NotContains(t, logs, "secret_prefix")
	require.NotContains(t, logs, "decrypted_prefix")
	require.Contains(t, logs, "secret_len=16")
}

type totpLoggingCache struct {
	session  *TotpSetupSession
	attempts int
}

func (c *totpLoggingCache) GetSetupSession(context.Context, int64) (*TotpSetupSession, error) {
	if c.session == nil {
		return nil, errors.New("setup session missing")
	}
	return c.session, nil
}

func (*totpLoggingCache) SetSetupSession(context.Context, int64, *TotpSetupSession, time.Duration) error {
	return nil
}

func (c *totpLoggingCache) DeleteSetupSession(context.Context, int64) error {
	c.session = nil
	return nil
}

func (*totpLoggingCache) GetLoginSession(context.Context, string) (*TotpLoginSession, error) {
	return nil, errors.New("login session not configured")
}

func (*totpLoggingCache) SetLoginSession(context.Context, string, *TotpLoginSession, time.Duration) error {
	return nil
}

func (*totpLoggingCache) DeleteLoginSession(context.Context, string) error { return nil }

func (c *totpLoggingCache) IncrementVerifyAttempts(context.Context, int64) (int, error) {
	c.attempts++
	return c.attempts, nil
}

func (c *totpLoggingCache) GetVerifyAttempts(context.Context, int64) (int, error) {
	return c.attempts, nil
}

func (c *totpLoggingCache) ClearVerifyAttempts(context.Context, int64) error {
	c.attempts = 0
	return nil
}

func (*totpLoggingCache) SetStepUpGrant(context.Context, int64, string, time.Duration) error {
	return nil
}

func (*totpLoggingCache) HasStepUpGrant(context.Context, int64, string) (bool, error) {
	return false, nil
}

type totpLoggingEncryptor struct {
	secret string
}

func (e *totpLoggingEncryptor) Encrypt(string) (string, error) { return "encrypted-sentinel", nil }

func (e *totpLoggingEncryptor) Decrypt(string) (string, error) { return e.secret, nil }

type totpLoggingUserRepo struct {
	UserRepository
	user *User
}

func (r *totpLoggingUserRepo) GetByID(context.Context, int64) (*User, error) {
	if r.user == nil {
		return nil, errors.New("user missing")
	}
	return r.user, nil
}

func (r *totpLoggingUserRepo) UpdateTotpSecret(_ context.Context, _ int64, encryptedSecret *string) error {
	r.user.TotpSecretEncrypted = encryptedSecret
	return nil
}

func (r *totpLoggingUserRepo) EnableTotp(context.Context, int64) error {
	r.user.TotpEnabled = true
	return nil
}

type totpLoggingSettingRepo struct {
	values map[string]string
}

func (r *totpLoggingSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *totpLoggingSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (*totpLoggingSettingRepo) Set(context.Context, string, string) error { return nil }

func (*totpLoggingSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (*totpLoggingSettingRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (*totpLoggingSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (*totpLoggingSettingRepo) Delete(context.Context, string) error { return nil }
