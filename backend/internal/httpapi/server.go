package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
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

	"updockly/backend/internal/agents"
	"updockly/backend/internal/auth"
	"updockly/backend/internal/certs"
	"updockly/backend/internal/config"
	"updockly/backend/internal/containers"
	"updockly/backend/internal/history"
	"updockly/backend/internal/logging"
	"updockly/backend/internal/metrics"
	"updockly/backend/internal/settings"
	"updockly/backend/internal/vault"
)

// Server wires HTTP handlers together.
type Server struct {
	cfg       config.Config
	db        *gorm.DB
	vault     *vault.Vault
	router    *gin.Engine
	log       *slog.Logger
	jwtSecret []byte
	timezone  *time.Location
	startedAt time.Time

	lastRecapDate   string
	recapPrimed     bool
	offlineNotified map[string]bool
	offlineMu       sync.Mutex
	autoUpdateRun   atomic.Bool

	agentService     *agents.AgentService
	authService      *auth.AuthService
	containerService *containers.ContainerService
	certManager      *certs.CertManager

	historyService *history.Service
	metricsService *metrics.Service

	loginAttempts map[string]loginAttempt
	loginMu       sync.Mutex

	settingsStore *settings.Store
}

type loginAttempt struct {
	count        int
	lastAttempt  time.Time
	blockedUntil time.Time
}

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	stateTTL        = 10 * time.Minute
)

func New(cfg config.Config, db *gorm.DB) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)
	logger := logging.New(cfg.LogLevel)

	loc := time.Local
	if cfg.Timezone != "" {
		if parsed, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = parsed
		}
	}
	// Ensure the process uses the configured timezone for time.Now() and formatting.
	time.Local = loc

	vaultSvc := vault.NewVault(cfg.VaultKey, cfg.SecretKey, cfg.VaultKeyPrevious)

	// Use shared volume path for certs if available; do not generate elsewhere if absent
	certDir := "/etc/updockly/certs"
	ensureCerts := true
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		ensureCerts = false
	}
	certManager := certs.NewCertManager(
		filepath.Join(certDir, "server.crt"),
		filepath.Join(certDir, "server.key"),
		filepath.Join(certDir, "ca.crt"),
	)
	if ensureCerts {
		if err := certManager.EnsureCertificates(); err != nil {
			// Fail fast to avoid serving with broken TLS
			return nil, fmt.Errorf("failed to generate or load TLS certs: %w", err)
		}
	} else {
		logger.Warn("certificate directory missing; TLS assets not generated", "certDir", certDir)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(logging.Middleware(logger))

	srv := &Server{
		cfg:              cfg,
		db:               db,
		vault:            vaultSvc,
		router:           router,
		log:              logger,
		jwtSecret:        []byte(cfg.JWTSecret),
		timezone:         loc,
		startedAt:        time.Now(),
		recapPrimed:      false,
		offlineNotified:  make(map[string]bool),
		agentService:     agents.NewAgentService(db, cfg.AgentRequireIPBinding),
		authService:      auth.NewAuthService(db, vaultSvc, cfg.JWTSecret, cfg.SecretKey, cfg.JWTSecretPrevious),
		containerService: containers.NewContainerService(db),
		certManager:      certManager,
		loginAttempts:    make(map[string]loginAttempt),
		historyService:   history.NewService(db),
		metricsService:   metrics.NewService(db, loc),
		settingsStore:    settings.NewStore(db),
	}

	srv.configureMiddleware()
	srv.registerRoutes()

	// If we have an existing database, try to re-encrypt secrets with the primary vault key.
	if db != nil {
		srv.reencryptVaultSecrets()
	}

	// Overlay persisted settings stored in the database on top of env defaults.
	srv.applyRuntimeSettings(srv.currentRuntimeSettings())

	return srv, nil
}

func (s *Server) configureMiddleware() {
	origins := make([]string, 0)
	if s.cfg.ClientOrigin != "" {
		for _, origin := range strings.Split(s.cfg.ClientOrigin, ",") {
			trimmed := strings.TrimSpace(origin)
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
				s.log.Warn("rejecting invalid CLIENT_ORIGIN entry; scheme required", "origin", trimmed)
				continue
			}
			origins = append(origins, strings.TrimSuffix(trimmed, "/"))
		}
	}
	// If no valid origins, default to none to avoid over-broad CORS.
	if len(origins) == 0 {
		s.log.Warn("no valid CLIENT_ORIGIN provided; CORS will reject cross-origin requests")
	}

	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Agent-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
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
		auth.POST("/refresh", s.refreshHandler)
		auth.POST("/logout", s.authMiddleware(), s.logoutHandler)
		auth.POST("/reset-password", s.resetPasswordHandler)
		auth.POST("/forgot-password", s.forgotPasswordHandler)
		auth.POST("/reset-password-token", s.resetPasswordWithTokenHandler)
		auth.POST("/2fa/verify", s.verify2FAHandler)
		auth.POST("/2fa/reset/init", s.reset2FAInitHandler)
		auth.POST("/2fa/reset/finalize", s.reset2FAFinalizeHandler)
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

func (s *Server) issueSession(c *gin.Context, acc *Account) error {
	access, err := s.authService.IssueToken(*acc, "", accessTokenTTL)
	if err != nil {
		return err
	}
	refresh, err := s.authService.IssueRefreshToken(acc, refreshTokenTTL)
	if err != nil {
		return err
	}
	s.setAuthCookies(c, access, refresh)
	return nil
}

func (s *Server) setAuthCookies(c *gin.Context, access, refresh string) {
	secure := false
	if origin := strings.Split(strings.TrimSpace(s.cfg.ClientOrigin), ",")[0]; strings.HasPrefix(strings.ToLower(origin), "https://") {
		secure = true
	}
	c.SetSameSite(http.SameSiteLaxMode)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		MaxAge:   int(accessTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		MaxAge:   int(refreshTokenTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearAuthCookies(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) signState(value string) string {
	sum := sha256.Sum256(append([]byte(value), s.jwtSecret...))
	return hex.EncodeToString(sum[:])
}

func (s *Server) verifyState(stateParam, stateCookie string) bool {
	parts := strings.Split(stateCookie, ".")
	if len(parts) != 2 {
		return false
	}
	raw, sig := parts[0], parts[1]
	if subtle.ConstantTimeCompare([]byte(stateParam), []byte(stateCookie)) != 1 {
		return false
	}
	expected := s.signState(raw)
	return subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1
}

func (s *Server) loginKey(username, ip string) string {
	user := strings.ToLower(strings.TrimSpace(username))
	return fmt.Sprintf("%s|%s", user, ip)
}

func (s *Server) isLoginBlocked(key string) (time.Duration, bool) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	state, ok := s.loginAttempts[key]
	if !ok {
		return 0, false
	}
	now := time.Now()
	if state.blockedUntil.After(now) {
		return time.Until(state.blockedUntil), true
	}
	// Reset stale counters after inactivity
	if now.Sub(state.lastAttempt) > 30*time.Minute {
		delete(s.loginAttempts, key)
		return 0, false
	}
	return 0, false
}

func (s *Server) recordLoginFailure(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	state := s.loginAttempts[key]
	now := time.Now()
	state.lastAttempt = now
	state.count++
	if state.count >= 5 {
		state.blockedUntil = now.Add(15 * time.Minute)
		s.log.Warn("login throttle block",
			"key", key,
			"blocked_until", state.blockedUntil.Format(time.RFC3339),
			"failures", state.count,
		)
	}
	s.loginAttempts[key] = state
}

func (s *Server) clearLoginFailures(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	delete(s.loginAttempts, key)
}

// reencryptVaultSecrets migrates stored secrets to the primary vault key to complete rotation.
func (s *Server) reencryptVaultSecrets() {
	if s.db == nil || s.vault == nil {
		return
	}
	var accounts []Account
	if err := s.db.Select("id", "two_factor_secret").Find(&accounts).Error; err != nil {
		s.log.Warn("unable to load accounts for vault re-encryption", "error", err)
		return
	}
	for _, acc := range accounts {
		if acc.TwoFactorSecret == "" {
			continue
		}
		secret, usedPrimary, err := s.vault.DecryptWithInfo(acc.TwoFactorSecret)
		if err != nil {
			fmt.Printf("Warning: failed to decrypt 2FA secret for account %s: %v\n", acc.ID, err)
			continue
		}
		if usedPrimary {
			continue
		}
		enc, err := s.vault.Encrypt(secret)
		if err != nil {
			fmt.Printf("Warning: failed to re-encrypt 2FA secret for account %s: %v\n", acc.ID, err)
			continue
		}
		if err := s.db.Model(&Account{}).Where("id = ?", acc.ID).Update("two_factor_secret", enc).Error; err != nil {
			fmt.Printf("Warning: failed to update rotated 2FA secret for account %s: %v\n", acc.ID, err)
		}
	}
}

func (s *Server) currentRuntimeSettings() config.RuntimeSettings {
	base := config.CurrentRuntimeSettings()
	if s.settingsStore == nil || s.db == nil {
		return base
	}
	stored, found, err := s.settingsStore.Load()
	if err != nil {
		s.log.Warn("failed to load runtime settings from database; falling back to env", "error", err)
		return base
	}
	if !found {
		return base
	}
	return settings.Merge(base, stored)
}

func (s *Server) saveRuntimeSettings(payload config.RuntimeSettings) (config.RuntimeSettings, error) {
	if s.settingsStore == nil || s.db == nil {
		return config.RuntimeSettings{}, fmt.Errorf("settings store unavailable")
	}

	base := s.currentRuntimeSettings()
	normalized := settings.NormalizeForStorage(base, payload)
	stored, err := s.settingsStore.Save(normalized)
	if err != nil {
		return config.RuntimeSettings{}, err
	}
	return settings.Merge(config.CurrentRuntimeSettings(), stored), nil
}

func (s *Server) applyRuntimeSettings(runtimeSettings config.RuntimeSettings) {
	oldDBURL := s.cfg.DatabaseURL
	s.cfg.DatabaseURL = runtimeSettings.DatabaseURL
	s.cfg.ClientOrigin = runtimeSettings.ClientOrigin
	// SECRET_KEY is retained for legacy; JWT/Vault keys are sourced from environment.
	s.cfg.SecretKey = runtimeSettings.SecretKey
	s.cfg.HideSupportButton = runtimeSettings.HideSupport
	s.cfg.Timezone = runtimeSettings.Timezone
	s.cfg.AutoPruneImages = runtimeSettings.AutoPrune
	s.cfg.Notifications = runtimeSettings.Notifications
	s.cfg.SSO = runtimeSettings.SSO

	if s.cfg.SecretKey != "" {
		// Preserve existing vault/jwt keys; do not overwrite them with legacy secret.
		s.vault = vault.NewVault(s.cfg.VaultKey, s.cfg.SecretKey, s.cfg.VaultKeyPrevious)
		s.jwtSecret = []byte(s.cfg.JWTSecret)
	}

	if runtimeSettings.Timezone != "" {
		if loc, err := time.LoadLocation(runtimeSettings.Timezone); err == nil {
			s.timezone = loc
			time.Local = loc
		}
	}

	// If the database URL has changed or if s.db is nil (initial setup), attempt to connect.
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
					&settings.Record{},
				); err == nil {
					s.db = db
					s.agentService = agents.NewAgentService(db, s.cfg.AgentRequireIPBinding)
					s.authService = auth.NewAuthService(db, s.vault, s.cfg.JWTSecret, s.cfg.SecretKey, s.cfg.JWTSecretPrevious)
					s.containerService = containers.NewContainerService(db)
					s.historyService = history.NewService(db)
					s.metricsService = metrics.NewService(db, s.timezone)
					s.settingsStore = settings.NewStore(db)
					s.reencryptVaultSecrets()
				}
			}
		}
	}

	host, port, db := config.ParseDatabaseURL(runtimeSettings.DatabaseURL)
	if host != "" {
		s.cfg.DBHost = host
		s.cfg.DBPort = port
		s.cfg.DBName = db
	}

	s.reencryptVaultSecrets()
}
