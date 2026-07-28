package service

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain configures Gin's process-global mode before any parallel test starts.
// Individual tests must not mutate Gin mode because SetMode is not concurrency-safe.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
