package config

import (
	"os"
	"strings"
)

// SingleUserPrivateControlPlaneEnabled reports whether the process is running
// as a private single-operator gateway control plane. ExAPI fails closed: only
// an explicit false value enables the legacy multi-user/SaaS surface.
func SingleUserPrivateControlPlaneEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE"))) {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// SaaSFeaturesEnabled reports whether multi-user commercial surfaces should be
// registered. Private gateway mode deliberately omits those routes.
func SaaSFeaturesEnabled() bool {
	return !SingleUserPrivateControlPlaneEnabled()
}
