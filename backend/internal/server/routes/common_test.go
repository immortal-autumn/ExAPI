package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCommonHealthIsLivenessOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r, func(context.Context) error { return errors.New("database unavailable") })

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok"}`, response.Body.String())
}

func TestCommonReadyChecksDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name   string
		probe  ReadinessProbe
		status int
		body   string
	}{
		{name: "ready", probe: func(context.Context) error { return nil }, status: http.StatusOK, body: `{"status":"ready"}`},
		{name: "not ready", probe: func(context.Context) error { return errors.New("redis unavailable") }, status: http.StatusServiceUnavailable, body: `{"status":"not_ready"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			RegisterCommonRoutes(r, tc.probe)
			request := httptest.NewRequest(http.MethodGet, "/ready", nil)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.JSONEq(t, tc.body, response.Body.String())
		})
	}
}

func TestCommonRoutesExcludeLegacySetupAndTelemetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterCommonRoutes(r, func(context.Context) error { return nil })

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/setup/status", nil),
		httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", nil),
	} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		require.Equal(t, http.StatusNotFound, response.Code, request.URL.Path)
	}
}
