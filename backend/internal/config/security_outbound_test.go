package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestOutboundSecurityPolicyCharacterization(t *testing.T) {
	t.Run("normalizes enforced mode without changing safe overrides", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		t.Setenv("SECURITY_OUTBOUND_MODE", " EnFoRcE ")
		t.Setenv("SECURITY_URL_ALLOWLIST_ENABLED", "true")
		t.Setenv("SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP", "false")
		t.Setenv("SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS", "false")

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, SecurityOutboundModeEnforce, cfg.Security.OutboundMode)
		require.True(t, cfg.Security.URLAllowlist.Enabled)
		require.False(t, cfg.Security.URLAllowlist.AllowInsecureHTTP)
		require.False(t, cfg.Security.URLAllowlist.AllowPrivateHosts)
	})

	t.Run("enforced mode rejects a disabled allowlist", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		viper.Set("security.outbound_mode", SecurityOutboundModeEnforce)
		viper.Set("security.url_allowlist.enabled", false)

		_, err := Load()
		require.ErrorContains(t, err, "security.url_allowlist.enabled must be true when security.outbound_mode=enforce")
	})

	t.Run("enforced mode treats whitespace-only upstream hosts as empty", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		viper.Set("security.outbound_mode", SecurityOutboundModeEnforce)
		viper.Set("security.url_allowlist.enabled", true)
		viper.Set("security.url_allowlist.upstream_hosts", []string{" ", "\t"})

		_, err := Load()
		require.ErrorContains(t, err, "security.url_allowlist.upstream_hosts must not be empty when security.outbound_mode=enforce")
	})

	t.Run("compat mode preserves legacy permissive settings", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		viper.Set("security.outbound_mode", " COMPAT ")
		viper.Set("security.url_allowlist.enabled", false)
		viper.Set("security.url_allowlist.upstream_hosts", []string{})
		viper.Set("security.url_allowlist.allow_private_hosts", true)
		viper.Set("security.url_allowlist.allow_insecure_http", true)

		cfg, err := Load()
		require.NoError(t, err)
		require.Equal(t, SecurityOutboundModeCompat, cfg.Security.OutboundMode)
		require.False(t, cfg.Security.URLAllowlist.Enabled)
		require.Empty(t, cfg.Security.URLAllowlist.UpstreamHosts)
		require.True(t, cfg.Security.URLAllowlist.AllowPrivateHosts)
		require.True(t, cfg.Security.URLAllowlist.AllowInsecureHTTP)
	})

	t.Run("rejects an unknown mode with the stable error", func(t *testing.T) {
		resetViperWithJWTSecret(t)
		viper.Set("security.outbound_mode", "audit")

		_, err := Load()
		require.ErrorContains(t, err, "security.outbound_mode must be one of: compat/enforce")
	})
}
