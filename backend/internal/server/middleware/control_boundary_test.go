package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func controlBoundaryTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ControlBoundary(config.ServerConfig{
		ControlHosts:    []string{"100.97.17.1", "control.example:8027"},
		OperatorPeerIPs: []string{"100.97.17.25", "::1"},
	}))
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/v1/operator/me", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.POST("/api/v1/admin/settings", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func TestControlBoundaryAcceptsExactWireGuardPeerAndHost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/api/v1/operator/me", nil)
	request.Host = "100.97.17.1:8027"
	request.RemoteAddr = "100.97.17.25:42100"
	request.Header.Set(ControlRequestHeader, "1")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	recorder := httptest.NewRecorder()

	controlBoundaryTestRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestControlBoundaryHidesUnknownHostsAndPeers(t *testing.T) {
	for _, mutate := range []func(*http.Request){
		func(request *http.Request) { request.Host = "attacker.invalid" },
		func(request *http.Request) { request.RemoteAddr = "203.0.113.9:42100" },
	} {
		request := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/", nil)
		request.Host = "100.97.17.1:8027"
		request.RemoteAddr = "100.97.17.25:42100"
		mutate(request)
		recorder := httptest.NewRecorder()

		controlBoundaryTestRouter().ServeHTTP(recorder, request)

		require.Equal(t, http.StatusNotFound, recorder.Code)
	}
}

func TestControlBoundaryRequiresMarkerOnControlAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/api/v1/operator/me", nil)
	request.Host = "100.97.17.1:8027"
	request.RemoteAddr = "100.97.17.25:42100"
	recorder := httptest.NewRecorder()

	controlBoundaryTestRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "CONTROL_REQUEST_REQUIRED")
}

func TestControlBoundaryRequiresFetchMetadataOnControlAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://100.97.17.1:8027/api/v1/operator/me", nil)
	request.Host = "100.97.17.1:8027"
	request.RemoteAddr = "100.97.17.25:42100"
	request.Header.Set(ControlRequestHeader, "1")
	recorder := httptest.NewRecorder()

	controlBoundaryTestRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestControlBoundaryRejectsCrossOriginAndCrossSiteMutations(t *testing.T) {
	for _, mutate := range []func(*http.Request){
		func(request *http.Request) { request.Header.Set("Origin", "https://attacker.invalid") },
		func(request *http.Request) { request.Header.Set("Sec-Fetch-Site", "cross-site") },
		func(request *http.Request) { request.Header.Del("Origin") },
	} {
		request := httptest.NewRequest(http.MethodPost, "http://100.97.17.1:8027/api/v1/admin/settings", strings.NewReader("{}"))
		request.Host = "100.97.17.1:8027"
		request.RemoteAddr = "100.97.17.25:42100"
		request.Header.Set(ControlRequestHeader, "1")
		request.Header.Set("Origin", "http://100.97.17.1:8027")
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		request.Header.Set("Sec-Fetch-Mode", "cors")
		mutate(request)
		recorder := httptest.NewRecorder()

		controlBoundaryTestRouter().ServeHTTP(recorder, request)

		require.Equal(t, http.StatusForbidden, recorder.Code)
	}
}

func TestControlBoundaryAcceptsWebSocketSubprotocolMarker(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://control.example:8027/api/v1/operator/me", nil)
	request.Host = "control.example:8027"
	request.RemoteAddr = "[::1]:42100"
	request.Header.Set("Connection", "upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Protocol", "exapi-control")
	recorder := httptest.NewRecorder()

	controlBoundaryTestRouter().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
