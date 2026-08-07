package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const frameSrcRefreshTimeout = 5 * time.Second

// SetupControlRouter configures the private operator listener. No gateway or
// credential-issuing authentication routes are registered here.
func SetupControlRouter(
	r *gin.Engine,
	handlers *handler.Handlers,
	operatorAuth middleware2.OperatorAuthMiddleware,
	auditLog middleware2.AuditLogMiddleware,
	panelRateLimiter *middleware2.PanelRateLimiter,
	opsService *service.OpsService,
	settingService *service.SettingService,
	cfg *config.Config,
	db *sql.DB,
	redisClient *redis.Client,
) *gin.Engine {
	middleware2.SetIngressRejectRecorder(opsService)
	// 缓存 iframe 页面的 origin 列表，用于动态注入 CSP frame-src
	var cachedFrameOrigins atomic.Pointer[[]string]
	emptyOrigins := []string{}
	cachedFrameOrigins.Store(&emptyOrigins)

	refreshFrameOrigins := func() {
		ctx, cancel := context.WithTimeout(context.Background(), frameSrcRefreshTimeout)
		defer cancel()
		origins, err := settingService.GetFrameSrcOrigins(ctx)
		if err != nil {
			// 获取失败时保留已有缓存，避免 frame-src 被意外清空
			return
		}
		cachedFrameOrigins.Store(&origins)
	}
	refreshFrameOrigins() // 启动时初始化

	// The direct peer/Host boundary runs before CORS, the SPA, or any API.
	r.Use(middleware2.ControlBoundary(cfg.Server))
	r.Use(middleware2.ControlPeerContext())
	r.Use(middleware2.RequestLogger())
	r.Use(middleware2.Logger())
	r.Use(middleware2.CORS(cfg.CORS))
	r.Use(middleware2.SecurityHeaders(cfg.Security.CSP, func() []string {
		if p := cachedFrameOrigins.Load(); p != nil {
			return *p
		}
		return nil
	}))
	r.Use(middleware2.ServerTiming(cfg.Server.EnableServerTiming))

	// Serve embedded frontend with settings injection if available
	if web.HasEmbeddedFrontend() {
		frontendServer, err := web.NewFrontendServer(settingService)
		if err != nil {
			log.Printf("Warning: Failed to create frontend server with settings injection: %v, using legacy mode", err)
			r.Use(web.ServeEmbeddedFrontend())
			settingService.SetOnUpdateCallback(refreshFrameOrigins)
		} else {
			// Register combined callback: invalidate HTML cache + refresh frame origins
			settingService.SetOnUpdateCallback(func() {
				frontendServer.InvalidateCache()
				refreshFrameOrigins()
			})
			r.Use(frontendServer.Middleware())
		}
	} else {
		settingService.SetOnUpdateCallback(refreshFrameOrigins)
	}

	registerControlRoutes(r, handlers, operatorAuth, auditLog, panelRateLimiter, settingService, cfg, db, redisClient)

	return r
}

func SetupPublicRouter(
	r *gin.Engine,
	handlers *handler.Handlers,
	apiKeyAuth middleware2.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
	db *sql.DB,
	redisClient *redis.Client,
) *gin.Engine {
	middleware2.SetIngressRejectRecorder(opsService)
	r.Use(middleware2.RequestLogger())
	r.Use(middleware2.SessionBindingContext(cfg))
	r.Use(middleware2.Logger())
	r.Use(middleware2.CORS(cfg.CORS))
	r.Use(middleware2.SecurityHeaders(cfg.Security.CSP, nil))
	r.Use(middleware2.ServerTiming(false))

	routes.RegisterCommonRoutes(r, func(ctx context.Context) error {
		return probeReadiness(ctx, db, redisClient)
	})
	routes.RegisterGatewayRoutes(r, handlers, apiKeyAuth, apiKeyService, subscriptionService, opsService, settingService, compositeResolver, cfg)
	return r
}

func registerControlRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	operatorAuth middleware2.OperatorAuthMiddleware,
	auditLog middleware2.AuditLogMiddleware,
	panelRateLimiter *middleware2.PanelRateLimiter,
	settingService *service.SettingService,
	cfg *config.Config,
	db *sql.DB,
	redisClient *redis.Client,
) {
	routes.RegisterCommonRoutes(r, func(ctx context.Context) error {
		return probeReadiness(ctx, db, redisClient)
	})

	v1 := r.Group("/api/v1")
	routes.RegisterOperatorRoutes(v1, h, operatorAuth, auditLog, panelRateLimiter)
	routes.RegisterAdminRoutes(v1, h, operatorAuth, auditLog, privateOperatorStepUp(), settingService, panelRateLimiter)
}

func privateOperatorStepUp() middleware2.StepUpAuthMiddleware {
	return middleware2.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
}

func probeReadiness(ctx context.Context, db *sql.DB, redisClient *redis.Client) error {
	if db == nil {
		return errors.New("database unavailable")
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}

	var migrationsApplied bool
	if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations)").Scan(&migrationsApplied); err != nil {
		return fmt.Errorf("schema migrations check: %w", err)
	}
	if !migrationsApplied {
		return errors.New("schema migrations unavailable")
	}

	if redisClient == nil {
		return errors.New("redis unavailable")
	}
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}
