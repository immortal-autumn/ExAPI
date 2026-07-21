// Package routecontract centralizes HTTP path classification shared by the
// gateway-facing middleware layers. Keep this list in parity with registered
// gateway aliases and deployment proxy locations.
package routecontract

import "strings"

const (
	ModelsPath            = "/models"
	ResponsesPath         = "/responses"
	ChatCompletionsPath   = "/chat/completions"
	EmbeddingsPath        = "/embeddings"
	ImagesGenerationsPath = "/images/generations"
	ImagesEditsPath       = "/images/edits"
	VideosGenerationsPath = "/videos/generations"
	VideosStatusPath      = "/videos/:request_id"
	AlphaSearchPath       = "/alpha/search"
)

func IsPublicGatewayPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "/health" ||
		path == ModelsPath ||
		path == ChatCompletionsPath ||
		path == EmbeddingsPath ||
		path == ImagesGenerationsPath ||
		path == ImagesEditsPath ||
		path == VideosGenerationsPath ||
		path == AlphaSearchPath {
		return true
	}
	for _, prefix := range []string{"/v1", "/v1beta", "/backend-api/codex", "/antigravity", ResponsesPath} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	if !strings.HasPrefix(path, "/videos/") {
		return false
	}
	requestID := strings.TrimPrefix(path, "/videos/")
	return requestID != "" && !strings.Contains(requestID, "/")
}

// IsAPIPath reports whether path belongs to an API/control endpoint and must
// never fall through to the embedded single-page application.
func IsAPIPath(path string) bool {
	path = strings.TrimSpace(path)

	if path == "/health" ||
		path == ModelsPath ||
		path == ChatCompletionsPath ||
		path == EmbeddingsPath ||
		path == ImagesGenerationsPath ||
		path == ImagesEditsPath ||
		path == VideosGenerationsPath ||
		path == AlphaSearchPath {
		return true
	}

	for _, prefix := range []string{
		"/api",
		"/v1",
		"/v1beta",
		"/backend-api",
		"/antigravity",
		"/setup",
		ResponsesPath,
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}

	if !strings.HasPrefix(path, "/videos/") {
		return false
	}
	requestID := strings.TrimPrefix(path, "/videos/")
	return requestID != "" && !strings.Contains(requestID, "/")
}
