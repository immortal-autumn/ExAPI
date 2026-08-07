package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operatorReaderStub struct {
	operator *service.User
	err      error
}

func (stub operatorReaderStub) GetFirstAdmin(context.Context) (*service.User, error) {
	return stub.operator, stub.err
}

func TestOperatorAuthBindsSingletonWithoutCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(operatorAuth(operatorReaderStub{operator: &service.User{
		ID: 7, Email: "operator@example.invalid", Role: service.RoleAdmin,
		Status: service.StatusActive, Concurrency: 12,
	}}))
	router.GET("/me", func(c *gin.Context) {
		operator, ok := GetOperatorFromContext(c)
		require.True(t, ok)
		subject, ok := GetAuthSubjectFromContext(c)
		require.True(t, ok)
		require.Equal(t, int64(7), subject.UserID)
		require.Equal(t, service.AuditAuthMethodWireGuardPeer, c.GetString("auth_method"))
		c.JSON(http.StatusOK, gin.H{"id": operator.ID})
	})

	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"id":7}`, recorder.Body.String())
}

func TestOperatorAuthFailsClosedWithoutActiveAdmin(t *testing.T) {
	for _, stub := range []operatorReaderStub{
		{err: errors.New("database unavailable")},
		{operator: &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive}},
		{operator: &service.User{ID: 7, Role: service.RoleAdmin, Status: service.StatusDisabled}},
	} {
		router := gin.New()
		router.Use(operatorAuth(stub))
		router.GET("/me", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

		require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	}
}
