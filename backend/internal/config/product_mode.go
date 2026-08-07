package config

// SingleUserPrivateControlPlaneEnabled reports the only supported ExAPI
// deployment mode. The old environment-variable escape hatch was removed so
// commercial/customer routes cannot be re-enabled accidentally.
func SingleUserPrivateControlPlaneEnabled() bool {
	return true
}

// SaaSFeaturesEnabled is retained as a source-compatibility predicate while
// the commercial/customer product is removed from ExAPI.
func SaaSFeaturesEnabled() bool {
	return false
}
