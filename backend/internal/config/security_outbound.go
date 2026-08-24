package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

const (
	SecurityOutboundModeCompat  = "compat"
	SecurityOutboundModeEnforce = "enforce"
)

// URLAllowlistConfig defines the compatibility and allowlist boundaries for
// outbound URLs. Field names and mapstructure keys are part of the existing
// configuration contract and intentionally remain unchanged.
type URLAllowlistConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	UpstreamHosts     []string `mapstructure:"upstream_hosts"`
	PricingHosts      []string `mapstructure:"pricing_hosts"`
	CRSHosts          []string `mapstructure:"crs_hosts"`
	AllowPrivateHosts bool     `mapstructure:"allow_private_hosts"`
	// 关闭 URL 白名单校验时，是否允许 http URL（默认只允许 https）
	AllowInsecureHTTP bool `mapstructure:"allow_insecure_http"`
}

func setOutboundSecurityDefaults() {
	viper.SetDefault("security.outbound_mode", SecurityOutboundModeCompat)
	viper.SetDefault("security.url_allowlist.enabled", false)
	viper.SetDefault("security.url_allowlist.upstream_hosts", []string{
		"api.openai.com",
		"api.anthropic.com",
		"api.kimi.com",
		"api.moonshot.ai",
		"api.moonshot.cn",
		"open.bigmodel.cn",
		"api.minimaxi.com",
		"generativelanguage.googleapis.com",
		"cloudcode-pa.googleapis.com",
		"*.openai.azure.com",
	})
	viper.SetDefault("security.url_allowlist.pricing_hosts", []string{
		"raw.githubusercontent.com",
	})
	viper.SetDefault("security.url_allowlist.crs_hosts", []string{})
	viper.SetDefault("security.url_allowlist.allow_private_hosts", true)
	viper.SetDefault("security.url_allowlist.allow_insecure_http", true)
}

func validateOutboundSecurityPolicy(security *SecurityConfig) error {
	security.OutboundMode = strings.ToLower(strings.TrimSpace(security.OutboundMode))
	switch security.OutboundMode {
	case SecurityOutboundModeCompat:
		return nil
	case SecurityOutboundModeEnforce:
		if !security.URLAllowlist.Enabled {
			return fmt.Errorf("security.url_allowlist.enabled must be true when security.outbound_mode=enforce")
		}
		if len(normalizeStringSlice(security.URLAllowlist.UpstreamHosts)) == 0 {
			return fmt.Errorf("security.url_allowlist.upstream_hosts must not be empty when security.outbound_mode=enforce")
		}
		return nil
	default:
		return fmt.Errorf("security.outbound_mode must be one of: compat/enforce")
	}
}

func warnOutboundSecurityCompatibility(security SecurityConfig) {
	if security.OutboundMode == SecurityOutboundModeCompat {
		slog.Warn("security.outbound_mode=compat; legacy outbound URL behavior remains enabled; migrate to enforce")
	}
	if !security.URLAllowlist.Enabled {
		slog.Warn("security.url_allowlist.enabled=false; allowlist/SSRF checks disabled (minimal format validation only)")
	}
}
