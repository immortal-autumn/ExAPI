package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/securitysecret"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	securitySecretKeyJWT        = "jwt_secret"
	securitySecretReadRetryMax  = 5
	securitySecretReadRetryWait = 10 * time.Millisecond
)

var readRandomBytes = rand.Read

func loadSecuritySecretProtector() (*securitySecretProtector, error) {
	keyrings, err := config.LoadExternalKeyringsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load external data-encryption keyring: %w", err)
	}
	protector, err := newSecuritySecretProtector(keyrings.DataEncryption)
	if err != nil {
		return nil, err
	}
	return protector, nil
}

func ensureBootstrapSecrets(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	if client == nil {
		return fmt.Errorf("nil ent client")
	}
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	protector, err := loadSecuritySecretProtector()
	if err != nil {
		return fmt.Errorf("protect bootstrap secrets: %w", err)
	}

	cfg.JWT.Secret = strings.TrimSpace(cfg.JWT.Secret)
	if cfg.JWT.Secret != "" {
		storedSecret, err := createSecuritySecretIfAbsentProtected(ctx, client, protector, securitySecretKeyJWT, cfg.JWT.Secret)
		if err != nil {
			return fmt.Errorf("persist jwt secret: %w", err)
		}
		if storedSecret != cfg.JWT.Secret {
			log.Println("Warning: configured JWT secret mismatches persisted value; using persisted secret for cross-instance consistency.")
		}
		cfg.JWT.Secret = storedSecret
		return nil
	}

	secret, created, err := getOrCreateGeneratedSecuritySecretProtected(ctx, client, protector, securitySecretKeyJWT, 32)
	if err != nil {
		return fmt.Errorf("ensure jwt secret: %w", err)
	}
	cfg.JWT.Secret = secret

	if created {
		log.Println("Warning: JWT secret auto-generated and persisted to database. Consider rotating to a managed secret for production.")
	}
	return nil
}

func getOrCreateGeneratedSecuritySecret(ctx context.Context, client *ent.Client, key string, byteLength int) (string, bool, error) {
	protector, err := loadSecuritySecretProtector()
	if err != nil {
		return "", false, err
	}
	return getOrCreateGeneratedSecuritySecretProtected(ctx, client, protector, key, byteLength)
}

func getOrCreateGeneratedSecuritySecretProtected(ctx context.Context, client *ent.Client, protector *securitySecretProtector, key string, byteLength int) (string, bool, error) {
	existing, err := client.SecuritySecret.Query().Where(securitysecret.KeyEQ(key)).Only(ctx)
	if err == nil {
		value, err := openAndMigrateSecuritySecret(ctx, client, protector, existing)
		if err != nil {
			return "", false, err
		}
		value = strings.TrimSpace(value)
		if len([]byte(value)) < 32 {
			return "", false, fmt.Errorf("stored secret %q must be at least 32 bytes", key)
		}
		return value, false, nil
	}
	if !ent.IsNotFound(err) {
		return "", false, err
	}

	generated, err := generateHexSecret(byteLength)
	if err != nil {
		return "", false, err
	}
	sealed, err := protector.seal(key, generated)
	if err != nil {
		return "", false, err
	}

	if err := client.SecuritySecret.Create().
		SetKey(key).
		SetValue(sealed).
		OnConflictColumns(securitysecret.FieldKey).
		DoNothing().
		Exec(ctx); err != nil {
		if !isSQLNoRowsError(err) {
			return "", false, err
		}
	}

	stored, err := querySecuritySecretWithRetry(ctx, client, key)
	if err != nil {
		return "", false, err
	}
	value, err := openAndMigrateSecuritySecret(ctx, client, protector, stored)
	if err != nil {
		return "", false, err
	}
	value = strings.TrimSpace(value)
	if len([]byte(value)) < 32 {
		return "", false, fmt.Errorf("stored secret %q must be at least 32 bytes", key)
	}
	return value, value == generated, nil
}

func createSecuritySecretIfAbsent(ctx context.Context, client *ent.Client, key, value string) (string, error) {
	protector, err := loadSecuritySecretProtector()
	if err != nil {
		return "", err
	}
	return createSecuritySecretIfAbsentProtected(ctx, client, protector, key, value)
}

func createSecuritySecretIfAbsentProtected(ctx context.Context, client *ent.Client, protector *securitySecretProtector, key, value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]byte(value)) < 32 {
		return "", fmt.Errorf("secret %q must be at least 32 bytes", key)
	}
	sealed, err := protector.seal(key, value)
	if err != nil {
		return "", err
	}

	if err := client.SecuritySecret.Create().
		SetKey(key).
		SetValue(sealed).
		OnConflictColumns(securitysecret.FieldKey).
		DoNothing().
		Exec(ctx); err != nil {
		if !isSQLNoRowsError(err) {
			return "", err
		}
	}

	stored, err := querySecuritySecretWithRetry(ctx, client, key)
	if err != nil {
		return "", err
	}
	storedValue, err := openAndMigrateSecuritySecret(ctx, client, protector, stored)
	if err != nil {
		return "", err
	}
	storedValue = strings.TrimSpace(storedValue)
	if len([]byte(storedValue)) < 32 {
		return "", fmt.Errorf("stored secret %q must be at least 32 bytes", key)
	}
	return storedValue, nil
}

func legacySecuritySecretMigrationEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_MIGRATE_LEGACY_SECURITY_SECRETS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func openAndMigrateSecuritySecret(ctx context.Context, client *ent.Client, protector *securitySecretProtector, stored *ent.SecuritySecret) (string, error) {
	if stored == nil {
		return "", errors.New("nil stored security secret")
	}
	current := stored
	for attempt := 0; attempt < 3; attempt++ {
		plaintext, rewrite, err := protector.open(current.Key, current.Value)
		if err != nil {
			return "", err
		}
		plaintext = strings.TrimSpace(plaintext)
		if len([]byte(plaintext)) < 32 {
			return "", fmt.Errorf("stored secret %q must be at least 32 bytes", current.Key)
		}
		if !rewrite {
			return plaintext, nil
		}
		// Legacy plaintext remains readable but is not rewritten automatically:
		// older binaries would interpret an envelope as the signing secret after
		// rollback. Operators enable this only after the rollback window closes.
		if !strings.HasPrefix(strings.TrimSpace(current.Value), securitySecretEnvelopePrefix) && !legacySecuritySecretMigrationEnabled() {
			return plaintext, nil
		}

		sealed, err := protector.seal(current.Key, plaintext)
		if err != nil {
			return "", err
		}
		updated, err := client.SecuritySecret.Update().
			Where(
				securitysecret.KeyEQ(current.Key),
				securitysecret.ValueEQ(current.Value),
			).
			SetValue(sealed).
			Save(ctx)
		if err != nil {
			return "", fmt.Errorf("rewrite security-secret envelope: %w", err)
		}
		if updated == 1 {
			return plaintext, nil
		}

		current, err = querySecuritySecretWithRetry(ctx, client, current.Key)
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("security secret %q changed repeatedly during envelope migration", stored.Key)
}

func querySecuritySecretWithRetry(ctx context.Context, client *ent.Client, key string) (*ent.SecuritySecret, error) {
	var lastErr error
	for attempt := 0; attempt <= securitySecretReadRetryMax; attempt++ {
		stored, err := client.SecuritySecret.Query().Where(securitysecret.KeyEQ(key)).Only(ctx)
		if err == nil {
			return stored, nil
		}
		if !isSecretNotFoundError(err) {
			return nil, err
		}
		lastErr = err
		if attempt == securitySecretReadRetryMax {
			break
		}

		timer := time.NewTimer(securitySecretReadRetryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isSecretNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return ent.IsNotFound(err) || isSQLNoRowsError(err)
}

func isSQLNoRowsError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows in result set")
}

func generateHexSecret(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	buf := make([]byte, byteLength)
	if _, err := readRandomBytes(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
