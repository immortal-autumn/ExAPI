package repository

import (
	"context"
	"errors"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	enthook "github.com/Wei-Shaw/sub2api/ent/hook"
	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

const (
	proxyPasswordPurpose         = "proxies.password"
	paymentProviderConfigPurpose = "payment_provider_instances.config"
)

// installSecretBridge centralizes dual-read/new-write protection for models
// whose legacy fields are read throughout the application. Callers continue to
// see plaintext in-memory, while mutations persist only authenticated envelopes
// in the additive columns.
func installSecretBridge(client *dbent.Client, keyring *secretcrypto.Keyring) error {
	if client == nil {
		return errors.New("nil ent client")
	}
	if keyring == nil {
		return errors.New("data-encryption keyring is required for secret bridge")
	}

	client.Proxy.Use(proxyPasswordProtectionHook(keyring))
	client.Proxy.Intercept(secretReadInterceptor(func(value dbent.Value) error {
		return openProxySecretValue(keyring, value)
	}))
	client.PaymentProviderInstance.Use(paymentProviderConfigProtectionHook(keyring))
	client.PaymentProviderInstance.Intercept(secretReadInterceptor(func(value dbent.Value) error {
		return openPaymentProviderSecretValue(keyring, value)
	}))
	return nil
}

func secretReadInterceptor(open func(dbent.Value) error) dbent.Interceptor {
	return dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(ctx context.Context, query dbent.Query) (dbent.Value, error) {
			value, err := next.Query(ctx, query)
			if err != nil {
				return nil, err
			}
			if err := open(value); err != nil {
				return nil, err
			}
			return value, nil
		})
	})
}

func proxyPasswordProtectionHook(keyring *secretcrypto.Keyring) dbent.Hook {
	return func(next dbent.Mutator) dbent.Mutator {
		return enthook.ProxyFunc(func(ctx context.Context, mutation *dbent.ProxyMutation) (dbent.Value, error) {
			if plaintext, ok := mutation.Password(); ok {
				envelope, err := keyring.Encrypt(proxyPasswordPurpose, []byte(plaintext))
				if err != nil {
					return nil, fmt.Errorf("protect proxy password: %w", err)
				}
				mutation.SetPasswordEncrypted(envelope)
				mutation.ClearPassword()
			} else if mutation.PasswordCleared() {
				mutation.ClearPasswordEncrypted()
			}
			value, err := next.Mutate(ctx, mutation)
			if err != nil {
				return nil, err
			}
			if err := openProxySecretValue(keyring, value); err != nil {
				return nil, err
			}
			return value, nil
		})
	}
}

func paymentProviderConfigProtectionHook(keyring *secretcrypto.Keyring) dbent.Hook {
	return func(next dbent.Mutator) dbent.Mutator {
		return enthook.PaymentProviderInstanceFunc(func(ctx context.Context, mutation *dbent.PaymentProviderInstanceMutation) (dbent.Value, error) {
			if plaintext, ok := mutation.Config(); ok {
				envelope, err := keyring.Encrypt(paymentProviderConfigPurpose, []byte(plaintext))
				if err != nil {
					return nil, fmt.Errorf("protect payment provider config: %w", err)
				}
				mutation.SetConfigEncrypted(envelope)
				// Config remains NOT NULL during the compatibility bridge.
				mutation.SetConfig("")
			}
			value, err := next.Mutate(ctx, mutation)
			if err != nil {
				return nil, err
			}
			if err := openPaymentProviderSecretValue(keyring, value); err != nil {
				return nil, err
			}
			return value, nil
		})
	}
}

func openProxySecretValue(keyring *secretcrypto.Keyring, value dbent.Value) error {
	switch typed := value.(type) {
	case *dbent.Proxy:
		return openProxySecret(keyring, typed)
	case []*dbent.Proxy:
		for _, item := range typed {
			if err := openProxySecret(keyring, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func openProxySecret(keyring *secretcrypto.Keyring, item *dbent.Proxy) error {
	if item == nil || item.PasswordEncrypted == nil {
		return nil
	}
	plaintext, err := keyring.Decrypt(proxyPasswordPurpose, *item.PasswordEncrypted)
	if err != nil {
		return fmt.Errorf("open proxy %d password: %w", item.ID, err)
	}
	password := string(plaintext)
	item.Password = &password
	return nil
}

func openPaymentProviderSecretValue(keyring *secretcrypto.Keyring, value dbent.Value) error {
	switch typed := value.(type) {
	case *dbent.PaymentProviderInstance:
		return openPaymentProviderSecret(keyring, typed)
	case []*dbent.PaymentProviderInstance:
		for _, item := range typed {
			if err := openPaymentProviderSecret(keyring, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func openPaymentProviderSecret(keyring *secretcrypto.Keyring, item *dbent.PaymentProviderInstance) error {
	if item == nil || item.ConfigEncrypted == nil {
		return nil
	}
	plaintext, err := keyring.Decrypt(paymentProviderConfigPurpose, *item.ConfigEncrypted)
	if err != nil {
		return fmt.Errorf("open payment provider instance %d config: %w", item.ID, err)
	}
	item.Config = string(plaintext)
	return nil
}
