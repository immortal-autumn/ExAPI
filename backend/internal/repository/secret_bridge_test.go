package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSecretBridgeWritesOnlyEnvelopesAndDualReadsLegacy(t *testing.T) {
	db, client := newSecretBridgeTestClient(t, "secret-bridge")

	key := []byte(strings.Repeat("k", 32))
	keyring, err := secretcrypto.NewKeyring("bridge-k1", map[string][]byte{"bridge-k1": key})
	require.NoError(t, err)
	require.NoError(t, installSecretBridge(client, keyring))

	ctx := context.Background()
	createdProxy, err := client.Proxy.Create().
		SetName("protected").SetProtocol("http").SetHost("127.0.0.1").SetPort(8080).
		SetPassword("proxy-secret").Save(ctx)
	require.NoError(t, err)
	require.NotNil(t, createdProxy.Password)
	require.Equal(t, "proxy-secret", *createdProxy.Password)

	var legacyPassword sql.NullString
	var proxyEnvelope sql.NullString
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT password, password_encrypted FROM proxies WHERE id = ?", createdProxy.ID).
		Scan(&legacyPassword, &proxyEnvelope))
	require.False(t, legacyPassword.Valid)
	require.True(t, proxyEnvelope.Valid)
	require.NotContains(t, proxyEnvelope.String, "proxy-secret")

	loadedProxy, err := client.Proxy.Get(ctx, createdProxy.ID)
	require.NoError(t, err)
	require.Equal(t, "proxy-secret", *loadedProxy.Password)

	createdProvider, err := client.PaymentProviderInstance.Create().
		SetProviderKey("stripe").SetName("protected").SetConfig(`{"secretKey":"sk-secret"}`).Save(ctx)
	require.NoError(t, err)
	require.Contains(t, createdProvider.Config, "sk-secret")

	var legacyConfig string
	var configEnvelope sql.NullString
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT config, config_encrypted FROM payment_provider_instances WHERE id = ?", createdProvider.ID).
		Scan(&legacyConfig, &configEnvelope))
	require.Empty(t, legacyConfig)
	require.True(t, configEnvelope.Valid)
	require.NotContains(t, configEnvelope.String, "sk-secret")

	loadedProvider, err := client.PaymentProviderInstance.Get(ctx, createdProvider.ID)
	require.NoError(t, err)
	require.Contains(t, loadedProvider.Config, "sk-secret")
}

func TestProtectedSettingsUseEnvelopeAndAdminKeyDigest(t *testing.T) {
	db, client := newSecretBridgeTestClient(t, "protected-settings")
	dataRing, err := secretcrypto.NewKeyring("data-k1", map[string][]byte{"data-k1": []byte(strings.Repeat("d", 32))})
	require.NoError(t, err)
	digestRing, err := secretcrypto.NewKeyring("digest-k1", map[string][]byte{"digest-k1": []byte(strings.Repeat("h", 32))})
	require.NoError(t, err)
	repo := &settingRepository{client: client, dataKeyring: dataRing, digestKeyring: digestRing}
	ctx := context.Background()

	require.NoError(t, repo.Set(ctx, service.SettingKeySMTPPassword, "smtp-secret"))
	value, err := repo.GetValue(ctx, service.SettingKeySMTPPassword)
	require.NoError(t, err)
	require.Equal(t, "smtp-secret", value)
	var envelope string
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT envelope FROM protected_settings WHERE key = ?", service.SettingKeySMTPPassword).Scan(&envelope))
	require.NotContains(t, envelope, "smtp-secret")
	var plaintextRows int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM settings WHERE key = ?", service.SettingKeySMTPPassword).Scan(&plaintextRows))
	require.Zero(t, plaintextRows)

	const adminKey = "admin-0123456789abcdef"
	require.NoError(t, repo.StoreAdminAPIKeyDigest(ctx, adminKey))
	matched, err := repo.VerifyAdminAPIKey(ctx, adminKey)
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = repo.VerifyAdminAPIKey(ctx, adminKey+"wrong")
	require.NoError(t, err)
	require.False(t, matched)
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT envelope FROM protected_settings WHERE key = ?", service.SettingKeyAdminAPIKey).Scan(&envelope))
	require.True(t, strings.HasPrefix(envelope, "hmac:v1:"))
	require.NotContains(t, envelope, adminKey)
}

func TestCutoverSecretBridgeRejectsDivergentOverlapsWithoutClearing(t *testing.T) {
	tests := []struct {
		name      string
		wantError string
		seed      func(context.Context, *sql.DB, *dbent.Client, *secretcrypto.Keyring, *secretcrypto.Keyring) func(*testing.T)
	}{
		{
			name:      "proxy password",
			wantError: "legacy value does not match protected envelope",
			seed: func(ctx context.Context, db *sql.DB, client *dbent.Client, dataRing, _ *secretcrypto.Keyring) func(*testing.T) {
				item, err := client.Proxy.Create().SetName("legacy").SetProtocol("http").SetHost("127.0.0.1").SetPort(8080).SetPassword("legacy-proxy").Save(ctx)
				require.NoError(t, err)
				envelope, err := dataRing.Encrypt(proxyPasswordPurpose, []byte("different-proxy"))
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `UPDATE proxies SET password_encrypted=? WHERE id=?`, envelope, item.ID)
				require.NoError(t, err)
				return func(t *testing.T) {
					var password sql.NullString
					require.NoError(t, db.QueryRowContext(ctx, `SELECT password FROM proxies WHERE id=?`, item.ID).Scan(&password))
					require.Equal(t, sql.NullString{String: "legacy-proxy", Valid: true}, password)
				}
			},
		},
		{
			name:      "payment config",
			wantError: "legacy value does not match protected envelope",
			seed: func(ctx context.Context, db *sql.DB, client *dbent.Client, dataRing, _ *secretcrypto.Keyring) func(*testing.T) {
				item, err := client.PaymentProviderInstance.Create().SetProviderKey("stripe").SetName("legacy").SetConfig(`{"key":"legacy"}`).Save(ctx)
				require.NoError(t, err)
				envelope, err := dataRing.Encrypt(paymentProviderConfigPurpose, []byte(`{"key":"different"}`))
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `UPDATE payment_provider_instances SET config_encrypted=? WHERE id=?`, envelope, item.ID)
				require.NoError(t, err)
				return func(t *testing.T) {
					var configValue string
					require.NoError(t, db.QueryRowContext(ctx, `SELECT config FROM payment_provider_instances WHERE id=?`, item.ID).Scan(&configValue))
					require.Equal(t, `{"key":"legacy"}`, configValue)
				}
			},
		},
		{
			name:      "data-envelope setting",
			wantError: "legacy value does not match protected envelope",
			seed: func(ctx context.Context, _ *sql.DB, client *dbent.Client, dataRing, _ *secretcrypto.Keyring) func(*testing.T) {
				const key = "smtp_password"
				_, err := client.Setting.Create().SetKey(key).SetValue("legacy-setting").Save(ctx)
				require.NoError(t, err)
				envelope, err := dataRing.Encrypt("settings."+key, []byte("different-setting"))
				require.NoError(t, err)
				_, err = client.ProtectedSetting.Create().SetKey(key).SetEnvelope(envelope).Save(ctx)
				require.NoError(t, err)
				return func(t *testing.T) {
					count, err := client.Setting.Query().Count(ctx)
					require.NoError(t, err)
					require.Equal(t, 1, count)
				}
			},
		},
		{
			name:      "admin API key digest",
			wantError: "legacy value does not match protected digest",
			seed: func(ctx context.Context, _ *sql.DB, client *dbent.Client, _ *secretcrypto.Keyring, digestRing *secretcrypto.Keyring) func(*testing.T) {
				_, err := client.Setting.Create().SetKey(service.SettingKeyAdminAPIKey).SetValue("legacy-admin-key").Save(ctx)
				require.NoError(t, err)
				digest, err := digestRing.Digest(adminAPIKeyDigestPurpose, []byte("different-admin-key"))
				require.NoError(t, err)
				_, err = client.ProtectedSetting.Create().SetKey(service.SettingKeyAdminAPIKey).SetEnvelope(digest).Save(ctx)
				require.NoError(t, err)
				return func(t *testing.T) {
					count, err := client.Setting.Query().Count(ctx)
					require.NoError(t, err)
					require.Equal(t, 1, count)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dataRing, digestRing := configureSecretBridgeMigrationKeyrings(t)
			db, client := newSecretBridgeTestClient(t, "cutover-divergence-"+strings.ReplaceAll(tc.name, " ", "-"))
			ctx := context.Background()
			assertLegacyRemains := tc.seed(ctx, db, client, dataRing, digestRing)

			err := CutoverSecretBridge(ctx, db)
			require.ErrorContains(t, err, tc.wantError)
			assertLegacyRemains(t)
		})
	}
}

func TestCutoverSecretBridgeClearsOnlyAfterMatchingSnapshotVerification(t *testing.T) {
	dataRing, digestRing := configureSecretBridgeMigrationKeyrings(t)
	db, client := newSecretBridgeTestClient(t, "cutover-matching")
	ctx := context.Background()

	proxy, err := client.Proxy.Create().SetName("legacy").SetProtocol("http").SetHost("127.0.0.1").SetPort(8080).SetPassword("proxy-secret").Save(ctx)
	require.NoError(t, err)
	proxyEnvelope, err := dataRing.Encrypt(proxyPasswordPurpose, []byte("proxy-secret"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE proxies SET password_encrypted=? WHERE id=?`, proxyEnvelope, proxy.ID)
	require.NoError(t, err)

	provider, err := client.PaymentProviderInstance.Create().SetProviderKey("stripe").SetName("legacy").SetConfig(`{"key":"payment-secret"}`).Save(ctx)
	require.NoError(t, err)
	providerEnvelope, err := dataRing.Encrypt(paymentProviderConfigPurpose, []byte(`{"key":"payment-secret"}`))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE payment_provider_instances SET config_encrypted=? WHERE id=?`, providerEnvelope, provider.ID)
	require.NoError(t, err)

	const settingKey = "smtp_password"
	_, err = client.Setting.Create().SetKey(settingKey).SetValue("setting-secret").Save(ctx)
	require.NoError(t, err)
	settingEnvelope, err := dataRing.Encrypt("settings."+settingKey, []byte("setting-secret"))
	require.NoError(t, err)
	_, err = client.ProtectedSetting.Create().SetKey(settingKey).SetEnvelope(settingEnvelope).Save(ctx)
	require.NoError(t, err)

	_, err = client.Setting.Create().SetKey(service.SettingKeyAdminAPIKey).SetValue("admin-secret").Save(ctx)
	require.NoError(t, err)
	adminDigest, err := digestRing.Digest(adminAPIKeyDigestPurpose, []byte("admin-secret"))
	require.NoError(t, err)
	_, err = client.ProtectedSetting.Create().SetKey(service.SettingKeyAdminAPIKey).SetEnvelope(adminDigest).Save(ctx)
	require.NoError(t, err)

	require.NoError(t, CutoverSecretBridge(ctx, db))

	var password sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `SELECT password FROM proxies WHERE id=?`, proxy.ID).Scan(&password))
	require.False(t, password.Valid)
	var configValue string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT config FROM payment_provider_instances WHERE id=?`, provider.ID).Scan(&configValue))
	require.Empty(t, configValue)
	legacySettings, err := client.Setting.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, legacySettings)
	protectedSettings, err := client.ProtectedSetting.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, protectedSettings)
}

func TestCutoverSecretBridgeCanResumeAfterCompletingProtection(t *testing.T) {
	dataRing, _ := configureSecretBridgeMigrationKeyrings(t)
	db, client := newSecretBridgeTestClient(t, "cutover-resume")
	ctx := context.Background()

	proxy, err := client.Proxy.Create().SetName("legacy").SetProtocol("http").SetHost("127.0.0.1").SetPort(8080).SetPassword("proxy-secret").Save(ctx)
	require.NoError(t, err)
	require.ErrorContains(t, CutoverSecretBridge(ctx, db), "legacy values remain without protected counterparts")

	var password sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `SELECT password FROM proxies WHERE id=?`, proxy.ID).Scan(&password))
	require.Equal(t, sql.NullString{String: "proxy-secret", Valid: true}, password)

	envelope, err := dataRing.Encrypt(proxyPasswordPurpose, []byte("proxy-secret"))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE proxies SET password_encrypted=? WHERE id=?`, envelope, proxy.ID)
	require.NoError(t, err)
	require.NoError(t, CutoverSecretBridge(ctx, db))

	require.NoError(t, db.QueryRowContext(ctx, `SELECT password FROM proxies WHERE id=?`, proxy.ID).Scan(&password))
	require.False(t, password.Valid)
}

func configureSecretBridgeMigrationKeyrings(t *testing.T) (*secretcrypto.Keyring, *secretcrypto.Keyring) {
	t.Helper()
	dataKey := []byte(strings.Repeat("d", 32))
	digestKey := []byte(strings.Repeat("h", 32))
	dataEncoded := base64.RawStdEncoding.EncodeToString(dataKey)
	digestEncoded := base64.RawStdEncoding.EncodeToString(digestKey)
	t.Setenv(config.EnvDataEncryptionActiveKeyID, "cutover-data-k1")
	t.Setenv(config.EnvDataEncryptionKeysJSON, fmt.Sprintf(`{"cutover-data-k1":%q}`, dataEncoded))
	t.Setenv(config.EnvGatewayKeyDigestActiveKeyID, "cutover-digest-k1")
	t.Setenv(config.EnvGatewayKeyDigestKeysJSON, fmt.Sprintf(`{"cutover-digest-k1":%q}`, digestEncoded))
	t.Setenv(config.EnvBackupEncryptionActiveKeyID, "")
	t.Setenv(config.EnvBackupEncryptionKeysJSON, "")
	dataRing, err := secretcrypto.NewKeyring("cutover-data-k1", map[string][]byte{"cutover-data-k1": dataKey})
	require.NoError(t, err)
	digestRing, err := secretcrypto.NewKeyring("cutover-digest-k1", map[string][]byte{"cutover-digest-k1": digestKey})
	require.NoError(t, err)
	return dataRing, digestRing
}

func newSecretBridgeTestClient(t *testing.T, name string) (*sql.DB, *dbent.Client) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+name+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Schema.Create(context.Background()))
	return db, client
}
