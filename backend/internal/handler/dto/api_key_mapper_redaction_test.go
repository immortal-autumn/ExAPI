package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestAPIKeyFromServiceNeverSerializesReusableKeyMaterial(t *testing.T) {
	const raw = "test-reusable-key-material"
	mapped := APIKeyFromService(&service.APIKey{
		ID:        9,
		UserID:    2,
		Key:       raw,
		KeyPrefix: "test-reusabl",
		Name:      "operator key",
		Status:    service.StatusActive,
	})
	if mapped.Key != "" {
		t.Fatal("generic API-key mapper retained reusable key material")
	}
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" || containsJSONValue(encoded, raw) {
		t.Fatal("generic API-key JSON contains reusable key material")
	}
	if !containsJSONValue(encoded, "test-reusabl") {
		t.Fatal("generic API-key JSON omitted display prefix")
	}
}

func containsJSONValue(encoded []byte, value string) bool {
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return false
	}
	for _, item := range payload {
		if text, ok := item.(string); ok && text == value {
			return true
		}
	}
	return false
}
