package service

import (
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

var ginTestModeOnce sync.Once

// TestMain configures Gin's process-global mode before any parallel test starts.
// Individual tests must not mutate Gin mode because SetMode is not concurrency-safe.
func TestMain(m *testing.M) {
	ensureGinTestMode()
	os.Exit(m.Run())
}

// Gin's mode is process-global and its setter is not concurrency-safe. The
// package TestMain sets it once before parallel tests start; legacy individual
// test setup calls route through the same once-guarded helper.
func ensureGinTestMode() {
	ginTestModeOnce.Do(func() { gin.SetMode(gin.TestMode) })
}
