//go:build integration

package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appmiddleware "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const authRouteRedisImageTag = "redis:8.4-alpine"

func TestAuthRegisterRateLimitThresholdHitReturns429(t *testing.T) {
	ctx := context.Background()
	rdb := startAuthRouteRedis(t, ctx)

	// Legacy browser-auth routes are intentionally absent from private ExAPI, so
	// exercise their Redis-backed abuse-control policy on an isolated route
	// instead of adding Redis back to newAuthRoutesTestRouter.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	rateLimiter := appmiddleware.NewRateLimiter(rdb)
	const path = "/api/v1/auth/register"
	router.POST(path, rateLimiter.LimitWithOptions("auth-register", 5, time.Minute, appmiddleware.RateLimitOptions{
		FailureMode: appmiddleware.RateLimitFailClose,
	}), func(c *gin.Context) {
		c.Status(http.StatusBadRequest)
	})

	for i := 1; i <= 6; i++ {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.10:23456"

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i <= 5 {
			require.Equal(t, http.StatusBadRequest, w.Code, "第 %d 次请求应先进入业务校验", i)
			continue
		}
		require.Equal(t, http.StatusTooManyRequests, w.Code, "第 6 次请求应命中限流")
		require.Contains(t, w.Body.String(), "rate limit exceeded")
	}
}

func startAuthRouteRedis(t *testing.T, ctx context.Context) *redis.Client {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)

	redisContainer, err := tcredis.Run(ctx, authRouteRedisImageTag)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = redisContainer.Terminate(ctx)
	})

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort.Int()),
		DB:   0,
	})
	require.NoError(t, rdb.Ping(ctx).Err())
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb
}
