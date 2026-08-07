package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	stopProducersGrace       = 5 * time.Second
	drainConnectionsGrace    = 3 * time.Second
	httpDrainGrace           = 8 * time.Second
	drainWorkersGrace        = 7 * time.Second
	flushStateGrace          = 5 * time.Second
	closeServicesGrace       = 5 * time.Second
	closeInfrastructureGrace = 3 * time.Second
	shutdownParentSlack      = 4 * time.Second
	shutdownSequentialMax    = stopProducersGrace + drainConnectionsGrace + httpDrainGrace + drainWorkersGrace + flushStateGrace + closeServicesGrace + closeInfrastructureGrace
	shutdownTotalGrace       = shutdownSequentialMax + shutdownParentSlack
)

type shutdownPhase struct {
	once      sync.Once
	completed bool
	steps     func(context.Context) []cleanupStep
}

type shutdownCoordinator struct {
	producers      shutdownPhase
	connections    shutdownPhase
	workers        shutdownPhase
	flushers       shutdownPhase
	services       shutdownPhase
	infrastructure shutdownPhase
}

func (s *shutdownCoordinator) runPhase(parent context.Context, name string, timeout time.Duration, phase *shutdownPhase) bool {
	if s == nil || phase == nil {
		return true
	}
	phase.once.Do(func() {
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		results, completed := runCleanupParallel(ctx, phase.steps(ctx))
		for _, result := range results {
			if result.err != nil {
				log.Printf("[Shutdown:%s] %s failed: %v", name, result.name, result.err)
			} else {
				log.Printf("[Shutdown:%s] %s succeeded", name, result.name)
			}
		}
		phase.completed = completed
		if !completed {
			log.Printf("[Shutdown:%s] phase deadline exceeded", name)
		}
	})
	return phase.completed
}

func (s *shutdownCoordinator) StopProducers(ctx context.Context) bool {
	return s.runPhase(ctx, "producers", stopProducersGrace, &s.producers)
}

func (s *shutdownCoordinator) DrainConnections(ctx context.Context) bool {
	return s.runPhase(ctx, "connections", drainConnectionsGrace, &s.connections)
}

func (s *shutdownCoordinator) DrainWorkers(ctx context.Context) bool {
	return s.runPhase(ctx, "workers", drainWorkersGrace, &s.workers)
}

func (s *shutdownCoordinator) Flush(ctx context.Context) bool {
	return s.runPhase(ctx, "flush", flushStateGrace, &s.flushers)
}

func (s *shutdownCoordinator) Close(ctx context.Context, safeToCloseInfrastructure bool) bool {
	if !safeToCloseInfrastructure {
		log.Printf("[Shutdown] Skipping service and infrastructure close because active work may still reference them")
		return false
	}
	servicesComplete := s.runPhase(ctx, "services", closeServicesGrace, &s.services)
	if !servicesComplete {
		log.Printf("[Shutdown] Skipping Redis/Ent close because a service did not stop cleanly")
		return false
	}
	return s.runPhase(ctx, "infrastructure", closeInfrastructureGrace, &s.infrastructure)
}

func (s *shutdownCoordinator) Run(ctx context.Context) {
	producersComplete := s.StopProducers(ctx)
	connectionsComplete := s.DrainConnections(ctx)
	workersComplete := s.DrainWorkers(ctx)
	flushComplete := s.Flush(ctx)
	s.Close(ctx, producersComplete && connectionsComplete && workersComplete && flushComplete)
}

func provideCleanup(
	entClient *ent.Client,
	rdb *redis.Client,
	opsMetricsCollector *service.OpsMetricsCollector,
	opsAggregation *service.OpsAggregationService,
	opsAlertEvaluator *service.OpsAlertEvaluatorService,
	opsCleanup *service.OpsCleanupService,
	opsScheduledReport *service.OpsScheduledReportService,
	opsSystemLogSink *service.OpsSystemLogSink,
	opsService *service.OpsService,
	opsIngressReject *service.OpsIngressRejectAggregator,
	apiKeyService *service.APIKeyService,
	authCacheInvalidationWorker *service.AuthCacheInvalidationWorker,
	schedulerSnapshot *service.SchedulerSnapshotService,
	tokenRefresh *service.TokenRefreshService,
	accountExpiry *service.AccountExpiryService,
	proxyExpiry *service.ProxyExpiryService,
	subscriptionExpiry *service.SubscriptionExpiryService,
	usageCleanup *service.UsageCleanupService,
	idempotencyCleanup *service.IdempotencyCleanupService,
	batchImageCleanup *service.BatchImageCleanupService,
	batchImageWorker *service.BatchImageWorkerRuntime,
	pricing *service.PricingService,
	emailQueue *service.EmailQueueService,
	billingCache *service.BillingCacheService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	subscriptionService *service.SubscriptionService,
	oauth *service.OAuthService,
	openaiOAuth *service.OpenAIOAuthService,
	geminiOAuth *service.GeminiOAuthService,
	antigravityOAuth *service.AntigravityOAuthService,
	grokOAuth *service.GrokOAuthService,
	openAIGateway *service.OpenAIGatewayService,
	scheduledTestRunner *service.ScheduledTestRunnerService,
	backupSvc *service.BackupService,
	paymentOrderExpiry *service.PaymentOrderExpiryService,
	channelMonitorRunner *service.ChannelMonitorRunner,
	quotaFlusher *service.UserPlatformQuotaUsageFlusher,
	upstreamBillingProbe *service.UpstreamBillingProbeService,
	auditLog *service.AuditLogService,
	promptAudit *securityaudit.PromptService,
) *shutdownCoordinator {
	stop := func(name string, fn func()) cleanupStep {
		return cleanupStep{name: name, fn: func() error { fn(); return nil }}
	}
	coordinator := &shutdownCoordinator{}
	coordinator.producers.steps = func(ctx context.Context) []cleanupStep {
		return []cleanupStep{
			stop("OpsIngressRejectAggregator", func() {
				if opsIngressReject != nil {
					opsIngressReject.Stop()
				}
			}),
			stop("OpsRuntimeSettingsRefresh", func() {
				if opsService != nil {
					opsService.StopRuntimeSettingsRefresh()
				}
			}),
			stop("OpsScheduledReportService", func() {
				if opsScheduledReport != nil {
					opsScheduledReport.Stop()
				}
			}),
			stop("OpsCleanupService", func() {
				if opsCleanup != nil {
					opsCleanup.Stop()
				}
			}),
			stop("OpsAlertEvaluatorService", func() {
				if opsAlertEvaluator != nil {
					opsAlertEvaluator.Stop()
				}
			}),
			stop("OpsAggregationService", func() {
				if opsAggregation != nil {
					opsAggregation.Stop()
				}
			}),
			stop("OpsMetricsCollector", func() {
				if opsMetricsCollector != nil {
					opsMetricsCollector.Stop()
				}
			}),
			stop("SchedulerSnapshotService", func() {
				if schedulerSnapshot != nil {
					schedulerSnapshot.Stop()
				}
			}),
			stop("UsageCleanupService", func() {
				if usageCleanup != nil {
					usageCleanup.Stop()
				}
			}),
			stop("IdempotencyCleanupService", func() {
				if idempotencyCleanup != nil {
					idempotencyCleanup.Stop()
				}
			}),
			stop("BatchImageCleanupService", func() {
				if batchImageCleanup != nil {
					batchImageCleanup.Stop()
				}
			}),
			stop("TokenRefreshService", func() {
				if tokenRefresh != nil {
					tokenRefresh.Stop()
				}
			}),
			stop("AccountExpiryService", func() {
				if accountExpiry != nil {
					accountExpiry.Stop()
				}
			}),
			stop("ProxyExpiryService", func() {
				if proxyExpiry != nil {
					proxyExpiry.Stop()
				}
			}),
			stop("SubscriptionExpiryService", func() {
				if subscriptionExpiry != nil {
					subscriptionExpiry.Stop()
				}
			}),
			stop("PricingService", func() {
				if pricing != nil {
					pricing.Stop()
				}
			}),
			stop("ScheduledTestRunnerService", func() {
				if scheduledTestRunner != nil {
					scheduledTestRunner.Stop()
				}
			}),
			stop("PaymentOrderExpiryService", func() {
				if paymentOrderExpiry != nil {
					paymentOrderExpiry.Stop()
				}
			}),
			stop("ChannelMonitorRunner", func() {
				if channelMonitorRunner != nil {
					channelMonitorRunner.Stop()
				}
			}),
			stop("UpstreamBillingProbeService", func() {
				if upstreamBillingProbe != nil {
					upstreamBillingProbe.Stop()
				}
			}),
			{name: "BackupService", fn: func() error {
				if backupSvc != nil {
					return backupSvc.StopContext(ctx)
				}
				return nil
			}},
		}
	}
	coordinator.connections.steps = func(context.Context) []cleanupStep {
		return []cleanupStep{stop("OpenAIWSPool", func() {
			if openAIGateway != nil {
				openAIGateway.CloseOpenAIWSPool()
			}
		})}
	}
	coordinator.workers.steps = func(ctx context.Context) []cleanupStep {
		return []cleanupStep{
			stop("AuthCacheInvalidationWorker", func() {
				if authCacheInvalidationWorker != nil {
					authCacheInvalidationWorker.Stop()
				}
			}),
			stop("AuthCacheInvalidationSubscriber", func() {
				if apiKeyService != nil {
					apiKeyService.StopAuthCacheInvalidationSubscriber()
				}
			}),
			stop("BatchImageWorkerRuntime", func() {
				if batchImageWorker != nil {
					batchImageWorker.Stop()
				}
			}),
			stop("UsageRecordWorkerPool", func() {
				if usageRecordWorkerPool != nil {
					usageRecordWorkerPool.Stop()
				}
			}),
			stop("EmailQueueService", func() {
				if emailQueue != nil {
					emailQueue.Stop()
				}
			}),
			{name: "PromptAuditService", fn: func() error {
				if promptAudit != nil {
					return promptAudit.Shutdown(ctx)
				}
				return nil
			}},
		}
	}
	coordinator.flushers.steps = func(context.Context) []cleanupStep {
		return []cleanupStep{
			stop("OpsSystemLogSink", func() {
				if opsSystemLogSink != nil {
					opsSystemLogSink.Stop()
				}
			}),
			stop("AuditLogService", func() {
				if auditLog != nil {
					auditLog.Stop()
				}
			}),
			stop("UserPlatformQuotaUsageFlusher", func() {
				if quotaFlusher != nil {
					quotaFlusher.Stop()
				}
			}),
			stop("BillingCacheService", func() {
				if billingCache != nil {
					billingCache.Stop()
				}
			}),
		}
	}
	coordinator.services.steps = func(context.Context) []cleanupStep {
		return []cleanupStep{
			stop("SubscriptionService", func() {
				if subscriptionService != nil {
					subscriptionService.Stop()
				}
			}),
			stop("OAuthService", func() {
				if oauth != nil {
					oauth.Stop()
				}
			}),
			stop("OpenAIOAuthService", func() {
				if openaiOAuth != nil {
					openaiOAuth.Stop()
				}
			}),
			stop("GeminiOAuthService", func() {
				if geminiOAuth != nil {
					geminiOAuth.Stop()
				}
			}),
			stop("AntigravityOAuthService", func() {
				if antigravityOAuth != nil {
					antigravityOAuth.Stop()
				}
			}),
			stop("GrokOAuthService", func() {
				if grokOAuth != nil {
					grokOAuth.Stop()
				}
			}),
		}
	}
	coordinator.infrastructure.steps = func(context.Context) []cleanupStep {
		return []cleanupStep{
			{name: "SharedInfrastructure", fn: func() error {
				if rdb != nil {
					if err := rdb.Close(); err != nil {
						return err
					}
				}
				if entClient != nil {
					return entClient.Close()
				}
				return nil
			}},
		}
	}
	return coordinator
}
