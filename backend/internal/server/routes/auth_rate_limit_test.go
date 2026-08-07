package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAuthRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterAuthRoutes(
		v1,
		&handler.Handlers{
			Auth:    &handler.AuthHandler{},
			Setting: &handler.SettingHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
		nil,
	)

	return router
}

func TestLegacyAuthRoutesRemainAbsent(t *testing.T) {
	router := newAuthRoutesTestRouter()
	paths := []string{
		"/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/login/2fa",
		"/api/v1/auth/send-verify-code",
		"/api/v1/auth/oauth/pending/send-verify-code",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "203.0.113.10:12345"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, "path=%s", path)
	}
}
