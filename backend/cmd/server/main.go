package main

//go:generate go run github.com/google/wire/cmd/wire

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/brand"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/setup"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
)

//go:embed VERSION
var embeddedVersion string

// Build-time variables (can be set by ldflags)
var (
	Version   = ""
	Commit    = "unknown"
	Date      = "unknown"
	BuildType = "source" // "source" for manual builds, "release" for CI builds (set by ldflags)
)

func init() {
	// 如果 Version 已通过 ldflags 注入（例如 -X main.Version=...），则不要覆盖。
	if strings.TrimSpace(Version) != "" {
		return
	}

	// 默认从 embedded VERSION 文件读取版本号（编译期打包进二进制）。
	Version = strings.TrimSpace(embeddedVersion)
	if Version == "" {
		Version = "0.0.0-dev"
	}
}

// initLogger configures the default slog handler based on gin.Mode().
// In non-release mode, Debug level logs are enabled.
func main() {
	logger.InitBootstrap()
	defer logger.Sync()

	// Parse command line flags
	setupMode := flag.Bool("setup", false, "Run setup wizard in CLI mode")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		log.Printf("%s %s (commit: %s, built: %s)\n", brand.ProductName, Version, Commit, Date)
		return
	}

	// CLI setup mode
	if *setupMode {
		if err := setup.RunCLI(); err != nil {
			log.Fatalf("Setup failed: %v", err)
		}
		return
	}

	// Check if setup is needed
	if setup.NeedsSetup() {
		// Check if auto-setup is enabled (for Docker deployment)
		if setup.AutoSetupEnabled() {
			log.Println("Auto setup mode enabled...")
			if err := setup.AutoSetupFromEnv(); err != nil {
				log.Fatalf("Auto setup failed: %v", err)
			}
			// Continue to main server after auto-setup
		} else {
			log.Println("First run detected, starting setup wizard...")
			runSetupServer()
			return
		}
	}

	// Normal server mode
	runMainServer()
}

func runSetupServer() {
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.SetupControlPlaneGuard())
	r.Use(middleware.CORS(config.CORSConfig{}))
	r.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true, Policy: config.DefaultCSPPolicy}, nil))

	// Register setup routes
	setup.RegisterRoutes(r)

	// Serve embedded frontend if available
	if web.HasEmbeddedFrontend() {
		r.Use(web.ServeEmbeddedFrontend())
	}

	// Get server address from config.yaml or environment variables (SERVER_HOST, SERVER_PORT)
	// This allows users to run setup on a different address if needed
	addr := config.GetServerAddress()
	log.Printf("Setup wizard available at http://%s", addr)
	log.Printf("Complete the setup wizard to configure %s", brand.ProductName)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	server := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
		Protocols:         protocols,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("Failed to start setup server: %v", err)
	}
}

func runMainServer() {
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := logger.Init(logger.OptionsFromConfig(cfg.Log)); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	if cfg.RunMode == config.RunModeSimple {
		log.Println("⚠️  WARNING: Running in SIMPLE mode - billing and quota checks are DISABLED")
	}

	buildInfo := handler.BuildInfo{
		Version:   Version,
		BuildType: BuildType,
	}

	app, err := initializeApplication(buildInfo)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	shutdownFinished := false
	defer func() {
		if shutdownFinished || app.Cleanup == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTotalGrace)
		defer cancel()
		app.Cleanup.Run(ctx)
	}()
	if app.PromptAudit != nil {
		if err := app.PromptAudit.Start(context.Background()); err != nil {
			// Startup continues so unrelated APIs stay up. Fail-closed (unavailable)
			// applies only when a persisted blocking policy was observed; without
			// blocking intent, Prompt Audit stays ModeOff so the gateway remains
			// usable and administrators can still disable the feature (#4560).
			log.Printf("Prompt Audit started in degraded state: %v", err)
		}
	}

	if app.Servers == nil || app.Servers.Public == nil || app.Servers.Control == nil {
		log.Fatal("Failed to initialize split public/control listeners")
	}
	type managedServer struct {
		name    string
		server  *http.Server
		tracker *activeHandlerTracker
	}
	managed := []*managedServer{
		{name: "public", server: app.Servers.Public},
		{name: "control", server: app.Servers.Control},
	}
	type serveResult struct {
		name string
		err  error
	}
	serverErr := make(chan serveResult, len(managed))
	for _, current := range managed {
		current.tracker = newActiveHandlerTracker(current.server.Handler)
		current.server.Handler = current.tracker
		go func(item *managedServer) {
			serverErr <- serveResult{name: item.name, err: item.server.ListenAndServe()}
		}(current)
		log.Printf("%s listener started on %s", current.name, current.server.Addr)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	exitedServers := 0
	select {
	case <-quit:
	case result := <-serverErr:
		exitedServers++
		if result.err != nil && !errors.Is(result.err, http.ErrServerClosed) {
			log.Printf("%s listener stopped unexpectedly: %v", result.name, result.err)
		}
	}

	log.Println("Shutting down listeners...")
	// Phase 1: make readiness/admission fail before stopping any producer.
	for _, current := range managed {
		current.tracker.StopAccepting()
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTotalGrace)
	defer cancel()
	producersComplete := app.Cleanup == nil || app.Cleanup.StopProducers(ctx)
	connectionsComplete := app.Cleanup == nil || app.Cleanup.DrainConnections(ctx)

	// Phase 3: drain HTTP after producers are stopped. Hijacked upstream WS
	// pools were closed by DrainConnections above.
	httpCtx, httpCancel := context.WithTimeout(ctx, httpDrainGrace)
	var shutdownWG sync.WaitGroup
	for _, current := range managed {
		shutdownWG.Add(1)
		go func(item *managedServer) {
			defer shutdownWG.Done()
			if err := item.server.Shutdown(httpCtx); err != nil {
				log.Printf("%s listener forced to shutdown: %v", item.name, err)
				if closeErr := item.server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
					log.Printf("%s listener close failed: %v", item.name, closeErr)
				}
			}
		}(current)
	}
	shutdownWG.Wait()
	httpCancel()
	for exitedServers < len(managed) {
		select {
		case result := <-serverErr:
			exitedServers++
			if result.err != nil && !errors.Is(result.err, http.ErrServerClosed) {
				log.Printf("%s listener exited with error: %v", result.name, result.err)
			}
		case <-ctx.Done():
			log.Printf("Listeners did not exit before shutdown deadline")
			exitedServers = len(managed)
		}
	}
	handlersComplete := true
	for _, current := range managed {
		if !current.tracker.WaitContext(ctx) {
			log.Printf("%s listener handlers did not drain before shutdown deadline", current.name)
			handlersComplete = false
		}
	}

	// Phases 4-6: close queue admission and drain workers, flush buffered
	// state, then stop services and finally shared infrastructure.
	workersComplete := app.Cleanup == nil || app.Cleanup.DrainWorkers(ctx)
	flushComplete := app.Cleanup == nil || app.Cleanup.Flush(ctx)
	if app.Cleanup != nil {
		app.Cleanup.Close(ctx, producersComplete && connectionsComplete && handlersComplete && workersComplete && flushComplete)
	}
	shutdownFinished = true
	log.Println("Server exited")
}

type activeHandlerTracker struct {
	next http.Handler

	mu      sync.Mutex
	closing bool
	active  sync.WaitGroup
}

func newActiveHandlerTracker(next http.Handler) *activeHandlerTracker {
	return &activeHandlerTracker{next: next}
}

func (t *activeHandlerTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	if t.closing {
		t.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	t.active.Add(1)
	t.mu.Unlock()

	defer t.active.Done()
	t.next.ServeHTTP(w, r)
}

func (t *activeHandlerTracker) StopAccepting() {
	t.mu.Lock()
	t.closing = true
	t.mu.Unlock()
}

func (t *activeHandlerTracker) Wait() {
	t.active.Wait()
}

func (t *activeHandlerTracker) WaitContext(ctx context.Context) bool {
	done := make(chan struct{})
	go func() {
		t.active.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}
