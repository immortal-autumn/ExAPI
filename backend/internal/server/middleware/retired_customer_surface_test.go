package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRetiredCustomerSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	retired := []string{
		"/api/v1/auth/login",
		"/api/v1/auth/oauth/oidc/callback",
		"/api/v1/user/profile",
		"/api/v1/users/me",
		"/api/v1/redeem/history",
		"/api/v1/subscriptions/active",
		"/api/v1/model-plaza",
		"/api/v1/model-plaza/groups",
		"/api/v1/payment/webhook/stripe",
		"/api/v1/settings/email-unsubscribe",
		"/api/v1/admin/users/42",
		"/api/v1/admin/groups/42/subscriptions",
		"/api/v1/admin/groups/42/subscriptions/active",
		"/api/v1/admin/affiliates/users",
		"/api/v1/admin/payment/orders",
	}

	for _, path := range retired {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(RetiredCustomerSurface())
			router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusGone, recorder.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, retiredCustomerSurfaceCode, body["code"])
			require.NotEmpty(t, body["message"])
			require.NotEmpty(t, body["message_zh"])
		})
	}
}

func TestRetiredCustomerSurfaceDoesNotInterceptPrivateOrGatewayAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	retained := []string{
		"/health",
		"/ready",
		"/api/v1/operator/me",
		"/api/v1/settings/public",
		"/api/v1/keys",
		"/api/v1/groups/available",
		"/api/v1/usage",
		"/api/v1/channel-monitors",
		"/api/v1/admin/accounts",
		"/v1/chat/completions",
		"/v1/models",
	}

	for _, path := range retained {
		t.Run(path, func(t *testing.T) {
			router := gin.New()
			router.Use(RetiredCustomerSurface())
			router.Any("/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			require.Equal(t, http.StatusNoContent, recorder.Code)
		})
	}
}

func TestRetiredCustomerSurfaceUsesSegmentBoundaries(t *testing.T) {
	for _, path := range []string{
		"/api/v1/authentic",
		"/api/v1/userland",
		"/api/v1/users-export",
		"/api/v1/model-plaza-preview",
		"/api/v1/admin/groups/42/subscriptions-preview",
		"/api/v1/admin/groups//subscriptions",
		"/api/v1/payment-methods",
		"/api/v1/admin/users-export",
	} {
		require.False(t, isRetiredCustomerPath(path), path)
	}
}
