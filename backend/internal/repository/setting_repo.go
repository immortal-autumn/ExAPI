package repository

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/protectedsetting"
	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type settingProtectionClass string

const (
	settingClassDataEnvelope     settingProtectionClass = "data-envelope"
	settingClassDigest           settingProtectionClass = "digest"
	settingClassAlreadyProtected settingProtectionClass = "already-protected"
	settingClassForbiddenPrivate settingProtectionClass = "forbidden-private"

	adminAPIKeyDigestPurpose = "settings.admin_api_key"
)

// sensitiveSettingRegistry is the authoritative classification. It is
// intentionally explicit: secret handling must never be inferred from a key
// name. already-protected entries contain a separately encrypted secret field.
var sensitiveSettingRegistry = map[string]settingProtectionClass{
	"smtp_password":                    settingClassDataEnvelope,
	"turnstile_secret_key":             settingClassDataEnvelope,
	"linuxdo_connect_client_secret":    settingClassDataEnvelope,
	"dingtalk_connect_client_secret":   settingClassDataEnvelope,
	"wechat_connect_app_secret":        settingClassDataEnvelope,
	"wechat_connect_open_app_secret":   settingClassDataEnvelope,
	"wechat_connect_mp_app_secret":     settingClassDataEnvelope,
	"wechat_connect_mobile_app_secret": settingClassDataEnvelope,
	"oidc_connect_client_secret":       settingClassDataEnvelope,
	"github_oauth_client_secret":       settingClassDataEnvelope,
	"google_oauth_client_secret":       settingClassDataEnvelope,
	"web_search_emulation_config":      settingClassDataEnvelope,
	"admin_api_key":                    settingClassDigest,
	"backup_s3_config":                 settingClassAlreadyProtected,
	"image_storage_config":             settingClassAlreadyProtected,
}

type settingRepository struct {
	client        *ent.Client
	dataKeyring   *secretcrypto.Keyring
	digestKeyring *secretcrypto.Keyring
	keyringErr    error
}

func NewSettingRepository(client *ent.Client) service.SettingRepository {
	repo := &settingRepository{client: client}
	keyrings, err := config.LoadExternalKeyringsFromEnv()
	if err != nil {
		repo.keyringErr = err
		return repo
	}
	repo.dataKeyring = keyrings.DataEncryption
	repo.digestKeyring = keyrings.GatewayKeyDigest
	return repo
}

func (r *settingRepository) Get(ctx context.Context, key string) (*service.Setting, error) {
	if sensitiveSettingRegistry[key] == settingClassDigest {
		return nil, service.ErrSettingNotFound
	}
	if sensitiveSettingRegistry[key] == settingClassDataEnvelope {
		if protected, err := r.client.ProtectedSetting.Query().Where(protectedsetting.KeyEQ(key)).Only(ctx); err == nil {
			value, err := r.decryptSetting(key, protected.Envelope)
			if err != nil {
				return nil, err
			}
			return &service.Setting{ID: protected.ID, Key: key, Value: value, UpdatedAt: protected.UpdatedAt}, nil
		} else if !ent.IsNotFound(err) {
			return nil, err
		}
	}
	return getLegacySetting(ctx, r.client, key)
}

func getLegacySetting(ctx context.Context, client *ent.Client, key string) (*service.Setting, error) {
	m, err := client.Setting.Query().Where(setting.KeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, service.ErrSettingNotFound
		}
		return nil, err
	}
	return &service.Setting{ID: m.ID, Key: m.Key, Value: m.Value, UpdatedAt: m.UpdatedAt}, nil
}

func (r *settingRepository) GetValue(ctx context.Context, key string) (string, error) {
	item, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return item.Value, nil
}

func (r *settingRepository) Set(ctx context.Context, key, value string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.setWithClient(ctx, tx.Client(), key, value); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *settingRepository) setWithClient(ctx context.Context, client *ent.Client, key, value string) error {
	class := sensitiveSettingRegistry[key]
	switch class {
	case settingClassDataEnvelope:
		if r.keyringErr != nil {
			return r.keyringErr
		}
		if r.dataKeyring == nil {
			return errors.New("data-encryption keyring is required for protected settings")
		}
		envelope, err := r.dataKeyring.Encrypt("settings."+key, []byte(value))
		if err != nil {
			return fmt.Errorf("protect setting %q: %w", key, err)
		}
		if err := upsertProtectedSetting(ctx, client, key, envelope); err != nil {
			return err
		}
		_, err = client.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx)
		return err
	case settingClassDigest:
		return r.storeDigestWithClient(ctx, client, key, value)
	case settingClassForbiddenPrivate:
		return fmt.Errorf("setting %q is forbidden in private mode", key)
	case settingClassAlreadyProtected, "":
		now := time.Now()
		return client.Setting.Create().SetKey(key).SetValue(value).SetUpdatedAt(now).
			OnConflictColumns(setting.FieldKey).UpdateNewValues().Exec(ctx)
	default:
		return fmt.Errorf("unsupported setting protection class %q", class)
	}
}

func upsertProtectedSetting(ctx context.Context, client *ent.Client, key, envelope string) error {
	return client.ProtectedSetting.Create().SetKey(key).SetEnvelope(envelope).SetUpdatedAt(time.Now()).
		OnConflictColumns(protectedsetting.FieldKey).UpdateNewValues().Exec(ctx)
}

func (r *settingRepository) decryptSetting(key, envelope string) (string, error) {
	if r.keyringErr != nil {
		return "", r.keyringErr
	}
	if r.dataKeyring == nil {
		return "", errors.New("data-encryption keyring is required for protected settings")
	}
	plaintext, err := r.dataKeyring.Decrypt("settings."+key, envelope)
	if err != nil {
		return "", fmt.Errorf("open setting %q: %w", key, err)
	}
	return string(plaintext), nil
}

func (r *settingRepository) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		value, err := r.GetValue(ctx, key)
		if errors.Is(err, service.ErrSettingNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}

func (r *settingRepository) SetMultiple(ctx context.Context, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for key, value := range values {
		if err := r.setWithClient(ctx, tx.Client(), key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *settingRepository) GetAll(ctx context.Context) (map[string]string, error) {
	legacy, err := r.client.Setting.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(legacy))
	for _, item := range legacy {
		if sensitiveSettingRegistry[item.Key] == settingClassDigest {
			continue
		}
		result[item.Key] = item.Value
	}
	protected, err := r.client.ProtectedSetting.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range protected {
		if sensitiveSettingRegistry[item.Key] != settingClassDataEnvelope {
			continue
		}
		value, err := r.decryptSetting(item.Key, item.Envelope)
		if err != nil {
			return nil, err
		}
		result[item.Key] = value
	}
	return result, nil
}

func (r *settingRepository) Delete(ctx context.Context, key string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ProtectedSetting.Delete().Where(protectedsetting.KeyEQ(key)).Exec(ctx); err != nil {
		return err
	}
	if _, err := tx.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

// StoreAdminAPIKeyDigest and VerifyAdminAPIKey are intentionally outside the
// broad SettingRepository interface so test doubles for ordinary settings do
// not gain secret-verification responsibilities.
func (r *settingRepository) StoreAdminAPIKeyDigest(ctx context.Context, candidate string) error {
	return r.Set(ctx, service.SettingKeyAdminAPIKey, candidate)
}

func (r *settingRepository) storeDigestWithClient(ctx context.Context, client *ent.Client, key, candidate string) error {
	if r.keyringErr != nil {
		return r.keyringErr
	}
	if r.digestKeyring == nil {
		return errors.New("gateway-key digest keyring is required for admin API key")
	}
	digest, err := r.digestKeyring.Digest(adminAPIKeyDigestPurpose, []byte(candidate))
	if err != nil {
		return err
	}
	if err := upsertProtectedSetting(ctx, client, key, digest); err != nil {
		return err
	}
	_, err = client.Setting.Delete().Where(setting.KeyEQ(key)).Exec(ctx)
	return err
}

func (r *settingRepository) VerifyAdminAPIKey(ctx context.Context, candidate string) (bool, error) {
	item, err := r.client.ProtectedSetting.Query().Where(protectedsetting.KeyEQ(service.SettingKeyAdminAPIKey)).Only(ctx)
	if err == nil {
		if r.keyringErr != nil {
			return false, r.keyringErr
		}
		if r.digestKeyring == nil {
			return false, errors.New("gateway-key digest keyring is required for admin API key")
		}
		return r.digestKeyring.VerifyDigest(adminAPIKeyDigestPurpose, []byte(candidate), item.Envelope), nil
	}
	if !ent.IsNotFound(err) {
		return false, err
	}
	legacy, err := getLegacySetting(ctx, r.client, service.SettingKeyAdminAPIKey)
	if errors.Is(err, service.ErrSettingNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	matched := subtle.ConstantTimeCompare([]byte(candidate), []byte(legacy.Value)) == 1
	if matched {
		if err := r.StoreAdminAPIKeyDigest(ctx, candidate); err != nil {
			return false, err
		}
	}
	return matched, nil
}

func (r *settingRepository) HasAdminAPIKey(ctx context.Context) (bool, error) {
	exists, err := r.client.ProtectedSetting.Query().Where(protectedsetting.KeyEQ(service.SettingKeyAdminAPIKey)).Exist(ctx)
	if err != nil || exists {
		return exists, err
	}
	return r.client.Setting.Query().Where(setting.KeyEQ(service.SettingKeyAdminAPIKey)).Exist(ctx)
}
