package config

import "testing"

func TestSingleUserPrivateControlPlaneEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "one", value: "1", want: true},
		{name: "true mixed case", value: " TrUe ", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "on", value: "ON", want: true},
		{name: "zero cannot select a legacy mode", value: "0", want: true},
		{name: "false cannot select a legacy mode", value: "false", want: true},
		{name: "empty fails closed to private mode", value: "", want: true},
		{name: "unknown fails closed to private mode", value: "enabled", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", tt.value)
			if got := SingleUserPrivateControlPlaneEnabled(); got != tt.want {
				t.Fatalf("SingleUserPrivateControlPlaneEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaaSFeaturesEnabled(t *testing.T) {
	t.Setenv("SUB2API_SINGLE_USER_PRIVATE_CONTROL_PLANE", "true")
	if SaaSFeaturesEnabled() {
		t.Fatal("SaaSFeaturesEnabled() = true in private single-user mode")
	}

	if SaaSFeaturesEnabled() {
		t.Fatal("SaaSFeaturesEnabled() = true in private-only mode")
	}
}
