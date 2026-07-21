package routecontract

import "testing"

func TestIsPublicGatewayPath(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"/models",
		"/v1/models",
		"/responses",
		"/chat/completions",
		"/alpha/search",
		"/videos/request-123",
	}
	for _, path := range accepted {
		path := path
		t.Run("accept_"+path, func(t *testing.T) {
			t.Parallel()
			if !IsPublicGatewayPath(path) {
				t.Fatalf("expected public gateway path: %s", path)
			}
		})
	}

	for _, path := range []string{"/", "/admin/dashboard", "/models-extra", "/videos/request-123/extra"} {
		path := path
		t.Run("reject_"+path, func(t *testing.T) {
			t.Parallel()
			if IsPublicGatewayPath(path) {
				t.Fatalf("expected non-gateway path: %s", path)
			}
		})
	}
}

func TestIsAPIPath(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"/api/v1/settings",
		"/models",
		"/v1/models",
		"/v1beta/models/gemini:generateContent",
		"/backend-api/codex/responses",
		"/antigravity/v1/messages",
		"/setup/init",
		"/health",
		"/responses",
		"/responses/compact",
		"/chat/completions",
		"/embeddings",
		"/images/generations",
		"/images/edits",
		"/videos/generations",
		"/videos/request-123",
		"/videos/generations-fake", // valid :request_id route
	}
	for _, path := range accepted {
		path := path
		t.Run("accept_"+path, func(t *testing.T) {
			t.Parallel()
			if !IsAPIPath(path) {
				t.Fatalf("expected API path: %s", path)
			}
		})
	}

	rejected := []string{
		"/",
		"/admin/dashboard",
		"/responses-fake",
		"/chat/completions-extra",
		"/embeddings-old",
		"/images",
		"/images/generations-fake",
		"/images/edits/extra",
		"/videos",
		"/videos/request-123/extra",
		"/videos/generations/extra",
	}
	for _, path := range rejected {
		path := path
		t.Run("reject_"+path, func(t *testing.T) {
			t.Parallel()
			if IsAPIPath(path) {
				t.Fatalf("expected non-API path: %s", path)
			}
		})
	}
}
