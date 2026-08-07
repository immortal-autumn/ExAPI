package middleware

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const ContextKeyOperator = "exapi_operator"

// OperatorAuthMiddleware binds the singleton database operator to a request
// that has already passed ControlBoundary. It consumes no password, cookie,
// bearer token, refresh token, or browser-stored credential.
type OperatorAuthMiddleware gin.HandlerFunc

type operatorReader interface {
	GetFirstAdmin(context.Context) (*service.User, error)
}

func NewOperatorAuthMiddleware(userService *service.UserService) OperatorAuthMiddleware {
	return OperatorAuthMiddleware(operatorAuth(userService))
}

func operatorAuth(users operatorReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if users == nil {
			AbortWithError(c, http.StatusServiceUnavailable, "OPERATOR_UNAVAILABLE", "Private operator unavailable")
			return
		}
		operator, err := users.GetFirstAdmin(c.Request.Context())
		if err != nil || operator == nil || !operator.IsAdmin() || !operator.IsActive() {
			AbortWithError(c, http.StatusServiceUnavailable, "OPERATOR_UNAVAILABLE", "Private operator unavailable")
			return
		}

		c.Set(ContextKeyOperator, operator)
		c.Set(string(ContextKeyUser), AuthSubject{UserID: operator.ID, Concurrency: operator.Concurrency})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Set(ContextKeyAuthEmail, operator.Email)
		c.Set("auth_method", service.AuditAuthMethodWireGuardPeer)
		c.Next()
	}
}

func GetOperatorFromContext(c *gin.Context) (*service.User, bool) {
	value, exists := c.Get(ContextKeyOperator)
	if !exists {
		return nil, false
	}
	operator, ok := value.(*service.User)
	return operator, ok && operator != nil
}
