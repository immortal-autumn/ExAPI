package config

import (
	"os"
	"strings"
)

// SingleUserPrivateControlPlaneEnabled reports whether the process is running
// as a private single-operator gateway control plane.
func SingleUserPrivateControlPlaneEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// SaaSFeaturesEnabled reports whether multi-user commercial surfaces should be
// registered. Private gateway mode deliberately omits those routes.
func SaaSFeaturesEnabled() bool {
	return !SingleUserPrivateControlPlaneEnabled()
}
