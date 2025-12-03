package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"updockly/backend/internal/config"
)

// Server wires HTTP handlers together.
type Server struct {
	cfg       config.Config
	db        *gorm.DB
	vault     *Vault
	router    *gin.Engine
	jwtSecret []byte
	timezone  *time.Location
	startedAt time.Time

	lastRecapDate   string
	recapPrimed     bool
	offlineNotified map[string]bool
	offlineMu       sync.Mutex
	autoUpdateRun   atomic.Bool

	agentService     *AgentService
	authService      *AuthService
	containerService *ContainerService
	certManager      *CertManager
}

func New(cfg config.Config, db *gorm.DB) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	loc := time.Local
	if cfg.Timezone != "" {
		if parsed, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = parsed
		}
	}
	// Ensure the process uses the configured timezone for time.Now() and formatting.
	time.Local = loc

	vault := NewVault(cfg.SecretKey)

	// Use shared volume path for certs if available, otherwise default to current directory
	certDir := "/etc/updockly/certs"
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		certDir = "."
	}
	certManager := NewCertManager(
		filepath.Join(certDir, "server.crt"),
		filepath.Join(certDir, "server.key"),
		filepath.Join(certDir, "ca.crt"),
	)
	if err := certManager.EnsureCertificates(); err != nil {
		// Log error but don't fail startup, as we might be running without TLS intent initially
		// or just serving HTTP behind proxy that handles its own TLS.
		// But for "generate on the fly" feature, we try our best.
		fmt.Printf("Warning: failed to generate self-signed certs: %v\n", err)
	}

	srv := &Server{
		cfg:             cfg,
		db:              db,
		vault:           vault,
		router:          gin.Default(),
		jwtSecret:       []byte(cfg.SecretKey),
		timezone:        loc,
		startedAt:       time.Now(),
		recapPrimed:     false,
		offlineNotified: make(map[string]bool),
		agentService:     NewAgentService(db),
		authService:      NewAuthService(db, vault, cfg.SecretKey),
		containerService: NewContainerService(db),
		certManager:      certManager,
	}

	srv.configureMiddleware()
	srv.registerRoutes()

	return srv, nil
}

func (s *Server) configureMiddleware() {
	origins := []string{"http://10.0.1.175:5173"}
	if s.cfg.ClientOrigin != "" {
		origins = append(origins, s.cfg.ClientOrigin)
	}

	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Agent-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOriginFunc:  func(origin string) bool { return true },
	}))

	// Security headers middleware
	s.router.Use(func(c *gin.Context) {
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
}

func (s *Server) registerRoutes() {
	s.router.GET("/health", s.healthHandler)
	s.router.GET("/api/health", s.healthHandler)

	auth := s.router.Group("/api/auth")
	{
		auth.POST("/login", s.loginHandler)
		auth.POST("/reset-password", s.resetPasswordHandler)
		auth.POST("/forgot-password", s.forgotPasswordHandler)
		auth.POST("/reset-password-token", s.resetPasswordWithTokenHandler)
		auth.POST("/2fa/verify", s.verify2FAHandler)
		auth.GET("/me", s.authMiddleware(), s.profileHandler)
		auth.PUT("/me", s.authMiddleware(), s.updateProfileHandler)
		auth.GET("/sso/login", s.ssoLoginHandler)
		auth.GET("/sso/callback", s.ssoCallbackHandler)
	}

	setup := s.router.Group("/api/auth/setup")
	{
		setup.GET("/status", s.setupStatusHandler)
		setup.GET("/runtime-settings", s.setupRuntimeSettingsHandler)
		setup.POST("/generate", s.setupGenerateHandler)
		setup.POST("/create", s.setupCreateHandler)
		setup.POST("/test-db", s.setupTestDbHandler)
	}

	api := s.router.Group("/api")
	api.POST("/agents/heartbeat", s.agentHeartbeatHandler)
	api.GET("/agents/commands/next", s.agentNextCommandHandler)
	api.POST("/agents/commands/:id/report", s.agentCommandReportHandler)
	api.GET("/metrics/running-history", s.runningHistoryHandler)
	api.Use(s.authMiddleware())
	{
		api.GET("/dashboard", s.dashboardHandler)
		api.GET("/containers/host-info", s.localHostInfo)
		api.GET("/containers", s.listContainers)
		api.POST("/containers/:id/check-update", s.checkContainerUpdateHandler)
		api.POST("/containers/:id/update", s.updateContainerHandler)
		api.POST("/containers/:id/rollback", s.rollbackContainerHandler)
		api.POST("/containers/:id/auto-update", s.toggleAutoUpdateHandler)
		api.POST("/containers/:id/start", s.startContainerHandler)
		api.POST("/containers/:id/stop", s.stopContainerHandler)
		api.POST("/containers/:id/restart", s.restartContainerHandler)
		api.GET("/containers/:id/logs", s.containerLogsHandler)
		api.GET("/containers/auto-update/count", s.countAutoUpdateContainers)
		api.GET("/history", s.listUpdateHistory)
		api.DELETE("/history/:id", s.deleteUpdateHistory)

		api.POST("/2fa/generate", s.generate2FAHandler)
		api.POST("/2fa/enable", s.enable2FAHandler)
		api.POST("/2fa/disable", s.disable2FAHandler)
		api.POST("/2fa/regenerate", s.regenerateRecoveryCodesHandler)

		api.GET("/settings", s.getSettings)
		api.PUT("/settings", s.updateSettings)
		api.POST("/notifications/test", s.testNotificationHandler)
		api.POST("/notifications/test-email", s.testEmailHandler)
		api.GET("/agents", s.listAgentsHandler)
		api.POST("/agents", s.createAgentHandler)
		api.PUT("/agents/:id", s.updateAgentHandler)
		api.POST("/agents/:id/rotate-token", s.rotateAgentTokenHandler)
		api.POST("/agents/:id/containers/:containerId/auto-update", s.toggleAgentContainerAutoUpdate)
		api.POST("/agents/:id/containers/:containerId/start", s.startAgentContainerHandler)
		api.POST("/agents/:id/containers/:containerId/stop", s.stopAgentContainerHandler)
		api.POST("/agents/:id/containers/:containerId/restart", s.restartAgentContainerHandler)
		api.POST("/agents/:id/containers/:containerId/rollback", s.rollbackAgentContainerHandler)
		api.Any("/agents/:id/containers/:containerId/logs", s.agentContainerLogsHandler)
		api.POST("/agents/:id/commands", s.createAgentCommandHandler)
		api.DELETE("/agents/:id", s.deleteAgentHandler)
		api.GET("/schedules", s.listSchedules)
		api.POST("/schedules", s.createSchedule)
		api.PUT("/schedules/:id", s.updateScheduleHandler)
		api.DELETE("/schedules/:id", s.deleteScheduleHandler)
		api.GET("/settings/ca-cert", s.downloadCACertHandler)
	}
}

func (s *Server) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.router,
	}

	go s.startNotificationScheduler(ctx)
	go s.startAutoUpdateScheduler(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}

func (s *Server) markAgentOfflineNotified(id string) bool {
	s.offlineMu.Lock()
	defer s.offlineMu.Unlock()
	if s.offlineNotified[id] {
		return false
	}
	s.offlineNotified[id] = true
	return true
}

func (s *Server) clearAgentOfflineNotified(id string) {
	s.offlineMu.Lock()
	defer s.offlineMu.Unlock()
	delete(s.offlineNotified, id)
}

func (s *Server) applyRuntimeSettings(settings config.RuntimeSettings) {
	oldDBURL := s.cfg.DatabaseURL
	s.cfg.DatabaseURL = settings.DatabaseURL
	s.cfg.ClientOrigin = settings.ClientOrigin
	s.cfg.SecretKey = settings.SecretKey
	s.cfg.Timezone = settings.Timezone
	s.cfg.AutoPruneImages = settings.AutoPrune
	s.cfg.Notifications = settings.Notifications
	s.cfg.SSO = settings.SSO

	if s.cfg.SecretKey != "" {
		s.vault = NewVault(s.cfg.SecretKey)
		s.jwtSecret = []byte(s.cfg.SecretKey)
	}

	if settings.Timezone != "" {
		if loc, err := time.LoadLocation(settings.Timezone); err == nil {
			s.timezone = loc
			time.Local = loc
		}
	}

	// If the database URL has changed or if s.db is nil (initial setup), attempt to connect
	if s.cfg.DatabaseURL != "" && (s.db == nil || s.cfg.DatabaseURL != oldDBURL) {
		var dial gorm.Dialector
		lower := strings.ToLower(s.cfg.DatabaseURL)
		switch {
		case strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://"):
			dial = postgres.Open(s.cfg.DatabaseURL)
		case strings.HasPrefix(lower, "sqlite://"):
			dsn := strings.TrimPrefix(s.cfg.DatabaseURL, "sqlite://")
			dial = sqlite.Open(dsn)
		}

		if dial != nil {
			if db, err := gorm.Open(dial, &gorm.Config{}); err == nil {
				if err := db.AutoMigrate(
					&Account{},
					&ContainerSettings{},
					&Schedule{},
					&Agent{},
					&AgentCommand{},
					&UpdateHistory{},
					&RunningSnapshot{},
				); err == nil {
					s.db = db
					s.agentService = NewAgentService(db)
					s.authService = NewAuthService(db, s.vault, s.cfg.SecretKey)
					s.containerService = NewContainerService(db)
				}
			}
		}
	}

	host, port, db := config.ParseDatabaseURL(settings.DatabaseURL)
	if host != "" {
		s.cfg.DBHost = host
		s.cfg.DBPort = port
		s.cfg.DBName = db
	}
}
