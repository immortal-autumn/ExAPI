package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const retiredCustomerSurfaceCode = "CUSTOMER_SURFACE_RETIRED"

var retiredCustomerSurfacePrefixes = []string{
	"/api/v1/auth",
	"/api/v1/user",
	"/api/v1/users",
	"/api/v1/channels",
	"/api/v1/model-plaza",
	"/api/v1/announcements",
	"/api/v1/redeem",
	"/api/v1/subscriptions",
	"/api/v1/payment",
	"/api/v1/settings/email-unsubscribe",
	"/api/v1/admin/users",
	"/api/v1/admin/announcements",
	"/api/v1/admin/redeem-codes",
	"/api/v1/admin/promo-codes",
	"/api/v1/admin/subscriptions",
	"/api/v1/admin/user-attributes",
	"/api/v1/admin/affiliates",
	"/api/v1/admin/payment",
}

// RetiredCustomerSurface gives former SaaS endpoints a stable response instead
// of allowing them to look temporarily missing. It intentionally does not
// match operator API-key, usage, group, or gateway routes.
func RetiredCustomerSurface() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isRetiredCustomerPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusGone, gin.H{
			"code":       retiredCustomerSurfaceCode,
			"message":    "This customer-facing feature has been removed from private ExAPI.",
			"message_zh": "此面向普通用户的功能已从私有 ExAPI 中移除。",
		})
	}
}

func isRetiredCustomerPath(path string) bool {
	for _, prefix := range retiredCustomerSurfacePrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
