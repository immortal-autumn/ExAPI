package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operatorBatchKeyLookup struct {
	key *service.APIKey
	err error
}

func (s operatorBatchKeyLookup) GetByID(context.Context, int64) (*service.APIKey, error) {
	return s.key, s.err
}

func runOperatorBatchKeyBinding(
	t *testing.T,
	query string,
	operatorID int64,
	lookup batchImageAPIKeyLookup,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &BatchImageHandler{apiKeys: lookup}
	router := gin.New()
	router.GET("/operator/batch-images", func(c *gin.Context) {
		c.Set(middleware.ContextKeyOperator, &service.User{ID: operatorID, Status: service.StatusActive})
		h.BindOperatorAPIKey(c)
	}, func(c *gin.Context) {
		key, ok := middleware.GetAPIKeyFromContext(c)
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{"api_key_id": key.ID})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/operator/batch-images"+query, nil)
	request.Header.Set("X-ExAPI-Control-Request", "1")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestBindOperatorAPIKeyRejectsMissingOrUnknownID(t *testing.T) {
	missing := runOperatorBatchKeyBinding(t, "", 7, operatorBatchKeyLookup{})
	require.Equal(t, http.StatusBadRequest, missing.Code)
	require.Contains(t, missing.Body.String(), "API_KEY_ID_REQUIRED")

	unknown := runOperatorBatchKeyBinding(t, "?api_key_id=42", 7, operatorBatchKeyLookup{err: errors.New("not found")})
	require.Equal(t, http.StatusNotFound, unknown.Code)
}

func TestBindOperatorAPIKeyRejectsAnotherOwnerAndUnavailableKeys(t *testing.T) {
	wrongOwner := runOperatorBatchKeyBinding(t, "?api_key_id=42", 7, operatorBatchKeyLookup{key: &service.APIKey{
		ID: 42, UserID: 8, Status: service.StatusActive,
	}})
	require.Equal(t, http.StatusNotFound, wrongOwner.Code)

	expiredAt := time.Now().Add(-time.Minute)
	cases := []struct {
		name string
		key  *service.APIKey
	}{
		{name: "inactive", key: &service.APIKey{ID: 42, UserID: 7, Status: service.StatusDisabled}},
		{name: "expired", key: &service.APIKey{ID: 42, UserID: 7, Status: service.StatusActive, ExpiresAt: &expiredAt}},
		{name: "exhausted", key: &service.APIKey{ID: 42, UserID: 7, Status: service.StatusActive, Quota: 1, QuotaUsed: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := runOperatorBatchKeyBinding(t, "?api_key_id=42", 7, operatorBatchKeyLookup{key: tc.key})
			require.Equal(t, http.StatusForbidden, response.Code)
			require.Contains(t, response.Body.String(), "API_KEY_UNAVAILABLE")
		})
	}
}

func TestBindOperatorAPIKeyUsesOpaqueIDWithoutRawCredential(t *testing.T) {
	const rawCredential = "sk-private-must-not-leak"
	response := runOperatorBatchKeyBinding(t, "?api_key_id=42", 7, operatorBatchKeyLookup{key: &service.APIKey{
		ID: 42, UserID: 7, Status: service.StatusActive, Key: rawCredential,
	}})

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"api_key_id":42}`, response.Body.String())
	require.False(t, strings.Contains(response.Body.String(), rawCredential))
}
