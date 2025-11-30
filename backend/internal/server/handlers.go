package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"golang.org/x/oauth2"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"updockly/backend/internal/config"
)

type testDbPayload struct {
	DatabaseURL string `json:"databaseUrl"`
}

func (s *Server) setupTestDbHandler(c *gin.Context) {
	if s.db != nil {
		var count int64
		s.db.Model(&Account{}).Count(&count)
		if count > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "setup already completed"})
			return
		}
	}

	var payload testDbPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var dial gorm.Dialector
	lower := strings.ToLower(payload.DatabaseURL)
	switch {
	case strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://"):
		dial = postgres.Open(payload.DatabaseURL)
	case strings.HasPrefix(lower, "sqlite://"):
		dsn := strings.TrimPrefix(payload.DatabaseURL, "sqlite://")
		dial = sqlite.Open(dsn)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid database URL scheme"})
		return
	}

	db, err := gorm.Open(dial, &gorm.Config{Logger: logger.Discard})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to open database: %v", err)})
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get sql.DB: %v", err)})
		return
	}

	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to ping database: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "database connection successful"})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) loginHandler(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	account, err := s.authService.Authenticate(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if account.TwoFactorEnabled {
		tempToken, err := s.authService.IssueToken(*account, "pre-2fa", 5*time.Minute)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to issue temporary token"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"twoFactorRequired": true,
			"tempToken":         tempToken,
		})
		return
	}

	token, err := s.authService.IssueToken(*account, "", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"username":         account.Username,
			"name":             account.Name,
			"role":             account.Role,
			"twoFactorEnabled": account.TwoFactorEnabled,
		},
	})
}

func (s *Server) profileHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}
	account, err := s.authService.GetAccount(claims.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"username":         account.Username,
		"name":             account.Name,
		"email":            account.Email,
		"role":             account.Role,
		"twoFactorEnabled": account.TwoFactorEnabled,
	})
}

type updateProfilePayload struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) updateProfileHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	var payload updateProfilePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if payload.NewPassword != "" && payload.CurrentPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password required to change password"})
		return
	}

	updated, err := s.authService.UpdateAccount(claims.Subject, payload.Name, payload.Email, payload.CurrentPassword, payload.NewPassword)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "invalid current password" || strings.Contains(err.Error(), "current password required") {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":         updated.Username,
		"name":             updated.Name,
		"email":            updated.Email,
		"role":             updated.Role,
		"twoFactorEnabled": updated.TwoFactorEnabled,
	})
}

func (s *Server) healthHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusOK, gin.H{"status": "OK", "version": "1.0.0", "time": time.Now().Format(time.RFC3339)})
		return
	}
	if err := s.db.Exec("SELECT 1").Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "OK", "version": "1.0.0", "time": time.Now().Format(time.RFC3339)})
}

func (s *Server) dashboardHandler(c *gin.Context) {
	totalContainers := 0
	runningContainers := 0
	autoUpdateEnabled := 0
	scheduleCount := int64(0)
	agentCount := int64(0)
	agentOnline := int64(0)
	message := "All systems are ready"
	now := time.Now()

	localAutoUpdate := make(map[string]bool)
	if s.db != nil {
		var settings []ContainerSettings
		if err := s.db.Find(&settings).Error; err == nil {
			for _, set := range settings {
				if set.AutoUpdate {
					localAutoUpdate[set.ID] = true
				}
			}
		}
		_ = s.db.Model(&Schedule{}).Count(&scheduleCount)
		_ = s.db.Model(&Agent{}).Count(&agentCount)
		_ = s.db.Model(&Agent{}).Where("last_seen > ?", now.Add(-5*time.Minute)).Count(&agentOnline)

		var agents []Agent
		if err := s.db.Find(&agents).Error; err == nil {
			for _, ag := range agents {
				if ag.LastSeen == nil || ag.LastSeen.Before(now.Add(-5*time.Minute)) {
					continue
				}
				for _, cont := range decodeContainers(ag) {
					totalContainers++
					if strings.ToLower(cont.State) == "running" {
						runningContainers++
					}
					if cont.AutoUpdate {
						autoUpdateEnabled++
					}
				}
			}
		}
	}

	if cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation()); err == nil {
		if containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true}); err == nil {
			totalContainers += len(containers)
			for _, cont := range containers {
				state := strings.ToLower(cont.State)
				status := strings.ToLower(cont.Status)
				if strings.Contains(state, "running") || strings.Contains(status, "up") {
					runningContainers++
				}
				if localAutoUpdate[cont.ID] {
					autoUpdateEnabled++
				}
			}
		} else {
			message = "Partial data; unable to query local Docker"
		}
	} else {
		message = "Partial data; unable to query local Docker"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           message,
		"time":              now.Format(time.RFC3339),
		"totalContainers":   totalContainers,
		"runningContainers": runningContainers,
		"autoUpdateEnabled": autoUpdateEnabled,
		"scheduleCount":     scheduleCount,
		"agentCount":        agentCount,
		"agentOnline":       agentOnline,
	})
}

type codePayload struct {
	Code string `json:"code"`
}

type verify2FAPayload struct {
	TempToken string `json:"tempToken"`
	Code      string `json:"code"`
}

func (s *Server) generate2FAHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	secret, qrCode, err := s.authService.Generate2FA(claims.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate 2fa"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"qrCode": qrCode,
	})
}

func (s *Server) enable2FAHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	var payload codePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	codes, err := s.authService.Enable2FA(claims.Subject, payload.Code)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "invalid 2fa code" || err.Error() == "2fa not set up" {
			status = http.StatusUnauthorized // or bad request, but original logic used unauthorized for invalid code
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "2fa enabled successfully",
		"recoveryCodes": codes,
	})
}

type disable2FAPayload struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

func (s *Server) disable2FAHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	var payload disable2FAPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := s.authService.Disable2FA(claims.Subject, payload.Code, payload.Password); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "invalid password" || err.Error() == "invalid 2fa code" {
			status = http.StatusUnauthorized
		} else if err.Error() == "2fa not enabled" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "2fa disabled successfully"})
}

func (s *Server) verify2FAHandler(c *gin.Context) {
	var payload verify2FAPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	claims, err := s.authService.VerifyToken(payload.TempToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid temporary token"})
		return
	}
	if claims.Type != "pre-2fa" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
		return
	}

	valid, err := s.authService.Validate2FA(claims.Subject, payload.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid 2fa code"})
		return
	}

	account, err := s.authService.GetAccount(claims.Subject)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}

	token, err := s.authService.IssueToken(*account, "", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to issue token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"username":         account.Username,
			"name":             account.Name,
			"role":             account.Role,
			"twoFactorEnabled": account.TwoFactorEnabled,
		},
	})
}

type resetPasswordPayload struct {
	Username     string `json:"username"`
	RecoveryCode string `json:"recoveryCode"`
	NewPassword  string `json:"newPassword"`
}

func (s *Server) resetPasswordHandler(c *gin.Context) {
	var payload resetPasswordPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if payload.Username == "" || payload.RecoveryCode == "" || payload.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required fields"})
		return
	}

	if err := s.authService.ResetPasswordWithRecoveryCode(payload.Username, payload.RecoveryCode, payload.NewPassword); err != nil {
		// Don't reveal if user exists vs code invalid for security (generic message)
		// But here invalid recovery code is specific enough.
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or recovery code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

type forgotPasswordPayload struct {
	Email string `json:"email"`
}

func (s *Server) forgotPasswordHandler(c *gin.Context) {
	var payload forgotPasswordPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if payload.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	token, account, err := s.authService.GeneratePasswordResetToken(payload.Email)
	if err != nil {
		// For security, do not reveal if email exists
		// Just return success or generic error if it's a system error
		// But here we can just simulate success if user not found to prevent enumeration
		if err.Error() == "user not found" {
			c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate reset token"})
		return
	}

	// Send Email
	if err := s.SendPasswordResetEmail(account.Email, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send email: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent"})
}

type resetPasswordWithTokenPayload struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (s *Server) resetPasswordWithTokenHandler(c *gin.Context) {
	var payload resetPasswordWithTokenPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if payload.Token == "" || payload.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token and new password are required"})
		return
	}

	if err := s.authService.ResetPasswordWithToken(payload.Token, payload.NewPassword); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

type setupGeneratePayload struct {
	SecretKey string `json:"secretKey"`
}

func (s *Server) setupStatusHandler(c *gin.Context) {
	needsSetup := true
	if s.db != nil {
		var count int64
		if err := s.db.Model(&Account{}).Count(&count).Error; err == nil {
			needsSetup = count == 0
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"needsSetup": needsSetup,
	})
}

func (s *Server) setupRuntimeSettingsHandler(c *gin.Context) {
	// Only allow reading runtime hints before the first admin exists.
	if s.db != nil {
		var count int64
		if err := s.db.Model(&Account{}).Count(&count).Error; err == nil && count > 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "setup already completed"})
			return
		}
	}

	settings := config.CurrentRuntimeSettings()
	c.JSON(http.StatusOK, gin.H{
		"databaseUrl": settings.DatabaseURL,
	})
}

func (s *Server) setupGenerateHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database connection not established"})
		return
	}

	exists, err := s.authService.AccountExists()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check setup status"})
		return
	}
	if exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "setup already completed"})
		return
	}

	var payload setupGeneratePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	secret := payload.SecretKey
	if secret == "" {
		secret = randomString(32)
	}

	settings := config.CurrentRuntimeSettings()
	settings.SecretKey = secret
	if err := config.SaveRuntimeSettings(config.EnvFilePath, settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.applyRuntimeSettings(settings)

	otpSecret, qrCode, err := s.authService.GenerateSetupOTP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": otpSecret,
		"qrCode": qrCode,
	})
}

type setupCreatePayload struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	Name       string `json:"name"`
	TOTPSecret string `json:"totpSecret"`
	TOTPCode   string `json:"totpCode"`
}

func (s *Server) setupCreateHandler(c *gin.Context) {
	if s.db != nil {
		exists, err := s.authService.AccountExists()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check setup status"})
			return
		}
		if exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "setup already completed"})
			return
		}
	}

	var payload setupCreatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	valid := totp.Validate(payload.TOTPCode, payload.TOTPSecret)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid 2fa code"})
		return
	}

	if s.db == nil {
		var dial gorm.Dialector
		lower := strings.ToLower(s.cfg.DatabaseURL)
		switch {
		case strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://"):
			dial = postgres.Open(s.cfg.DatabaseURL)
		case strings.HasPrefix(lower, "sqlite://"):
			dsn := strings.TrimPrefix(s.cfg.DatabaseURL, "sqlite://")
			dial = sqlite.Open(dsn)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid database URL scheme"})
			return
		}

		db, err := gorm.Open(dial, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to database"})
			return
		}

		if err := db.AutoMigrate(&Account{}, &Agent{}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to run migrations"})
			return
		}
		s.db = db
		s.applyRuntimeSettings(config.CurrentRuntimeSettings()) // This will init services
	}

	codes, err := s.authService.CreateAdmin(payload.Username, payload.Email, payload.Password, payload.Name, payload.TOTPSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create admin account"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "admin account created successfully",
		"recoveryCodes": codes,
	})
}

func (s *Server) getSettings(c *gin.Context) {
	settings := config.CurrentRuntimeSettings()
	c.JSON(http.StatusOK, settings)
}

func (s *Server) updateSettings(c *gin.Context) {
	var payload config.RuntimeSettings
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if payload.Timezone == "" {
		payload.Timezone = os.Getenv("TIMEZONE")
		if payload.Timezone == "" {
			payload.Timezone = "UTC"
		}
	}

	if err := config.SaveRuntimeSettings(config.EnvFilePath, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.applyRuntimeSettings(payload)
	c.JSON(http.StatusOK, payload)
}

func (s *Server) regenerateRecoveryCodesHandler(c *gin.Context) {
	claims := getClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	codes, err := s.authService.RegenerateRecoveryCodes(claims.Subject)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "2fa not enabled" {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recoveryCodes": codes,
	})
}

func (s *Server) ssoLoginHandler(c *gin.Context) {
	if !s.cfg.SSO.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "SSO is not enabled"})
		return
	}

	provider, err := oidc.NewProvider(context.Background(), s.cfg.SSO.IssuerURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get provider: %v", err)})
		return
	}

	oauth2Config := oauth2.Config{
		ClientID:     s.cfg.SSO.ClientID,
		ClientSecret: s.cfg.SSO.ClientSecret,
		RedirectURL:  s.cfg.SSO.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	state := randomString(16)
	// In a real app, store state in cookie/session to validate in callback
	c.SetCookie("oauth_state", state, 3600, "/", "", false, true)

	c.Redirect(http.StatusFound, oauth2Config.AuthCodeURL(state))
}

func (s *Server) ssoCallbackHandler(c *gin.Context) {
	if !s.cfg.SSO.Enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "SSO is not enabled"})
		return
	}

	state, err := c.Cookie("oauth_state")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state cookie not found"})
		return
	}
	if c.Query("state") != state {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
		return
	}

	provider, err := oidc.NewProvider(context.Background(), s.cfg.SSO.IssuerURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to get provider: %v", err)})
		return
	}

	oauth2Config := oauth2.Config{
		ClientID:     s.cfg.SSO.ClientID,
		ClientSecret: s.cfg.SSO.ClientSecret,
		RedirectURL:  s.cfg.SSO.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	token, err := oauth2Config.Exchange(context.Background(), c.Query("code"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to exchange token: %v", err)})
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no id_token field in oauth2 token"})
		return
	}

	idTokenVerifier := provider.Verifier(&oidc.Config{ClientID: s.cfg.SSO.ClientID})
	idToken, err := idTokenVerifier.Verify(context.Background(), rawIDToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to verify ID token: %v", err)})
		return
	}

	var claims struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to parse claims: %v", err)})
		return
	}

	// Determine unique identifier (prefer username, fallback to email)
	identifier := claims.Username
	if identifier == "" {
		identifier = claims.Email
	}

	// Check if user exists (case-insensitive lookup)
	account, err := s.authService.FindAccountForSSO(identifier)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		clientOrigin := strings.TrimSuffix(s.cfg.ClientOrigin, "/")
		redirectURL := fmt.Sprintf("%s/login?error=User+not+authorized", clientOrigin)
		c.Redirect(http.StatusFound, redirectURL)
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Issue internal token
	jwtToken, err := s.authService.IssueToken(*account, "sso", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue internal token"})
		return
	}

	// Redirect to frontend with token
	// Assuming frontend is served from same origin or we know the URL
	clientOrigin := strings.TrimSuffix(s.cfg.ClientOrigin, "/")
	redirectURL := fmt.Sprintf("%s/auth/callback?token=%s", clientOrigin, jwtToken)
	c.Redirect(http.StatusFound, redirectURL)
}

type agentPayload struct {
	Name       string `json:"name" binding:"required"`
	Hostname   string `json:"hostname"`
	Notes      string `json:"notes"`
	TLSEnabled bool   `json:"tlsEnabled"`
}

type agentResponse struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Hostname      string              `json:"hostname"`
	Notes         string              `json:"notes"`
	TLSEnabled    bool                `json:"tlsEnabled"`
	AgentVersion  string              `json:"agentVersion"`
	DockerVersion string              `json:"dockerVersion"`
	Platform      string              `json:"platform"`
	LastSeen      *time.Time          `json:"lastSeen,omitempty"`
	Token         string              `json:"token,omitempty"`
	Containers    []ContainerSnapshot `json:"containers,omitempty"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type ContainerSnapshot struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Image           string     `json:"image"`
	State           string     `json:"state"`
	Status          string     `json:"status"`
	AutoUpdate      bool       `json:"autoUpdate"`
	UpdateAvailable bool       `json:"updateAvailable"`
	CheckedAt       *time.Time `json:"checkedAt,omitempty"`
	Ports           []string   `json:"ports,omitempty"`
	Labels          []string   `json:"labels,omitempty"`
}

func decodeContainers(agent Agent) []ContainerSnapshot {
	if len(agent.Containers) == 0 {
		return nil
	}
	out := make([]ContainerSnapshot, len(agent.Containers))
	copy(out, agent.Containers)
	return out
}

func toAgentResponse(agent Agent, includeToken bool) agentResponse {
	resp := agentResponse{
		ID:            agent.ID,
		Name:          agent.Name,
		Hostname:      agent.Hostname,
		Notes:         agent.Notes,
		TLSEnabled:    agent.TLSEnabled,
		AgentVersion:  agent.AgentVersion,
		DockerVersion: agent.DockerVersion,
		Platform:      agent.Platform,
		LastSeen:      agent.LastSeen,
		Containers:    decodeContainers(agent),
		CreatedAt:     agent.CreatedAt,
		UpdatedAt:     agent.UpdatedAt,
	}
	if includeToken {
		resp.Token = agent.Token
	}
	return resp
}

func (s *Server) listAgentsHandler(c *gin.Context) {
	agents, err := s.agentService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agents"})
		return
	}
	out := make([]agentResponse, 0, len(agents))
	for _, agent := range agents {
		out = append(out, toAgentResponse(agent, false))
	}
	c.JSON(http.StatusOK, out)
}

func (s *Server) createAgentHandler(c *gin.Context) {
	var payload agentPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	agent, err := s.agentService.Create(payload.Name, payload.Hostname, payload.Notes, payload.TLSEnabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent"})
		return
	}
	c.JSON(http.StatusCreated, toAgentResponse(*agent, true))
}

func (s *Server) rotateAgentTokenHandler(c *gin.Context) {
	id := c.Param("id")
	agent, err := s.agentService.RotateToken(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rotate token"})
		return
	}
	c.JSON(http.StatusOK, toAgentResponse(*agent, true))
}

func (s *Server) updateAgentHandler(c *gin.Context) {
	id := c.Param("id")
	var payload agentPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if strings.TrimSpace(payload.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	agent, err := s.agentService.Update(id, payload.Name, payload.Hostname, payload.Notes, payload.TLSEnabled)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent"})
		return
	}

	c.JSON(http.StatusOK, toAgentResponse(*agent, false))
}

func (s *Server) downloadCACertHandler(c *gin.Context) {
	certBytes, err := s.certManager.GetCACert()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "CA certificate not found"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=ca.crt")
	c.Data(http.StatusOK, "application/x-x509-ca-cert", certBytes)
}

func (s *Server) deleteScheduleHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database connection not available"})
		return
	}
	id := c.Param("id")
	if err := s.db.Delete(&Schedule{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete schedule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule deleted"})
}

func (s *Server) deleteAgentHandler(c *gin.Context) {
	id := c.Param("id")
	if err := s.agentService.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete agent"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) startAgentContainerHandler(c *gin.Context) {
	agentID := c.Param("id")
	containerID := c.Param("containerId")
	if _, err := s.createAgentCommandInternal(agentID, "start-container", JSONMap{"containerId": containerID}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "start requested"})
}

func (s *Server) stopAgentContainerHandler(c *gin.Context) {
	agentID := c.Param("id")
	containerID := c.Param("containerId")
	if _, err := s.createAgentCommandInternal(agentID, "stop-container", JSONMap{"containerId": containerID}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "stop requested"})
}

func (s *Server) restartAgentContainerHandler(c *gin.Context) {
	agentID := c.Param("id")
	containerID := c.Param("containerId")
	if _, err := s.createAgentCommandInternal(agentID, "restart-container", JSONMap{"containerId": containerID}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "restart requested"})
}

func (s *Server) rollbackAgentContainerHandler(c *gin.Context) {
	agentID := c.Param("id")
	containerID := c.Param("containerId")
	var payload struct {
		Image     string `json:"image"`
		HistoryID string `json:"historyId,omitempty"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.Image) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	cmdPayload := JSONMap{
		"containerId": containerID,
		"image":       strings.TrimSpace(payload.Image),
	}
	if strings.TrimSpace(payload.HistoryID) != "" {
		cmdPayload["historyId"] = strings.TrimSpace(payload.HistoryID)
	}

	if _, err := s.createAgentCommandInternal(agentID, "rollback-container", cmdPayload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "rollback requested"})
}

func (s *Server) agentContainerLogsHandler(c *gin.Context) {
	agentID := c.Param("id")
	containerID := c.Param("containerId")
	if agentID == "" || containerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing agent or container id"})
		return
	}
	tailParam := strings.TrimSpace(c.DefaultQuery("tail", "200"))
	tail := 200
	if parsed, err := strconv.Atoi(tailParam); err == nil && parsed > 0 && parsed <= 2000 {
		tail = parsed
	}
	var cached string
	if logStr, _ := s.latestAgentLogs(agentID, containerID); logStr != "" {
		c.JSON(http.StatusOK, gin.H{"logs": logStr})
		return
	}

	if _, err := s.createAgentCommandInternal(agentID, "fetch-logs", JSONMap{"containerId": containerID, "tail": tail}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if logStr, _ := s.latestAgentLogs(agentID, containerID); logStr != "" {
			c.JSON(http.StatusOK, gin.H{"logs": logStr})
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":    cached,
		"message": "logs requested; awaiting agent response",
	})
}

func (s *Server) createAgentCommandInternal(agentID, cmdType string, payload JSONMap) (*AgentCommand, error) {
	return s.agentService.CreateCommand(agentID, cmdType, payload)
}

func (s *Server) latestAgentLogs(agentID, containerID string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not ready")
	}
	silent := s.db.Session(&gorm.Session{Logger: logger.Discard})
	var cmd AgentCommand
	err := silent.Where("agent_id = ? AND type = ? AND status = ? AND payload ->> 'containerId' = ?", agentID, "fetch-logs", "completed", containerID).
		Order("created_at DESC").First(&cmd).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if cmd.Result != nil {
		if logs, ok := cmd.Result["logs"].(string); ok {
			return logs, nil
		}
	}
	return "", nil
}

type agentHeartbeatPayload struct {
	Hostname      string              `json:"hostname"`
	AgentVersion  string              `json:"agentVersion"`
	DockerVersion string              `json:"dockerVersion"`
	Platform      string              `json:"platform"`
	Containers    []ContainerSnapshot `json:"containers"`
}

func (s *Server) agentHeartbeatHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}

	token := strings.TrimSpace(c.GetHeader("X-Agent-Token"))
	if token == "" {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		}
	}
	token = strings.Trim(strings.TrimSpace(token), "\"'")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
		return
	}

	var payload agentHeartbeatPayload
	if err := c.ShouldBindJSON(&payload); err != nil && err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var agent Agent
	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
	if err := silentDB.Where("token = ?", token).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent"})
		return
	}

	now := time.Now()
	agent.LastSeen = &now
	s.clearAgentOfflineNotified(agent.ID)
	updates := map[string]interface{}{
		"last_seen": now,
	}
	if payload.Hostname != "" {
		agent.Hostname = payload.Hostname
		updates["hostname"] = agent.Hostname
	}
	if payload.AgentVersion != "" {
		agent.AgentVersion = payload.AgentVersion
		updates["agent_version"] = agent.AgentVersion
	}
	if payload.DockerVersion != "" {
		agent.DockerVersion = payload.DockerVersion
		updates["docker_version"] = agent.DockerVersion
	}
	if payload.Platform != "" {
		agent.Platform = payload.Platform
		updates["platform"] = agent.Platform
	}
	if payload.Containers != nil {
		// Preserve per-container flags (like updateAvailable) across heartbeats
		existing := decodeContainers(agent)
		existingByID := make(map[string]ContainerSnapshot, len(existing))
		for _, c := range existing {
			existingByID[c.ID] = c
		}
		merged := make([]ContainerSnapshot, 0, len(payload.Containers))
		for _, c := range payload.Containers {
			if !c.UpdateAvailable {
				if prev, ok := existingByID[c.ID]; ok && prev.UpdateAvailable {
					c.UpdateAvailable = true
				}
			}
			if c.CheckedAt == nil {
				if prev, ok := existingByID[c.ID]; ok && prev.CheckedAt != nil {
					c.CheckedAt = prev.CheckedAt
				}
			}
			if !c.AutoUpdate {
				if prev, ok := existingByID[c.ID]; ok && prev.AutoUpdate {
					c.AutoUpdate = true
				}
			}
			merged = append(merged, c)
		}
		agent.Containers = merged
		updates["containers"] = agent.Containers
	}

	if err := silentDB.Model(&agent).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "heartbeat received"})
}

func (s *Server) getAgentByToken(c *gin.Context) (*Agent, bool) {
	token := strings.TrimSpace(c.GetHeader("X-Agent-Token"))
	if token == "" {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			token = strings.TrimSpace(authHeader[7:])
		}
	}
	token = strings.Trim(strings.TrimSpace(token), "\"'")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
		return nil, true
	}

	agent, err := s.agentService.GetByToken(token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return nil, true
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent"})
		return nil, true
	}
	return agent, false
}

type createAgentCommandPayload struct {
	Type        string `json:"type" binding:"required"`
	ContainerID string `json:"containerId" binding:"required"`
	Image       string `json:"image"`
}

func (s *Server) createAgentCommandHandler(c *gin.Context) {
	agentID := c.Param("id")
	var payload createAgentCommandPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	switch payload.Type {
	case "check-update", "update-container", "start-container", "stop-container", "restart-container", "fetch-logs":
	case "rollback-container":
		if strings.TrimSpace(payload.Image) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "rollback requires image"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported command type"})
		return
	}

	cmdPayload := JSONMap{"containerId": payload.ContainerID}
	if strings.TrimSpace(payload.Image) != "" {
		cmdPayload["image"] = strings.TrimSpace(payload.Image)
	}

	cmd, err := s.createAgentCommandInternal(agentID, payload.Type, cmdPayload)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          cmd.ID,
		"type":        cmd.Type,
		"status":      cmd.Status,
		"payload":     cmd.Payload,
		"createdAt":   cmd.CreatedAt,
		"agentId":     cmd.AgentID,
		"startedAt":   cmd.StartedAt,
		"completedAt": cmd.CompletedAt,
	})
}

func (s *Server) agentNextCommandHandler(c *gin.Context) {
	agent, handled := s.getAgentByToken(c)
	if handled {
		return
	}

	cmd, err := s.agentService.GetNextCommand(agent.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load next command"})
		return
	}
	if cmd == nil {
		c.Status(http.StatusNoContent)
		return
	}

	now := time.Now()
	cmd.Status = "running"
	cmd.StartedAt = &now

	if err := s.agentService.UpdateCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark command running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      cmd.ID,
		"type":    cmd.Type,
		"payload": cmd.Payload,
	})
}

type agentCommandReportPayload struct {
	Status string  `json:"status" binding:"required"`
	Result JSONMap `json:"result"`
	Error  string  `json:"error"`
}

func (s *Server) agentCommandReportHandler(c *gin.Context) {
	agent, handled := s.getAgentByToken(c)
	if handled {
		return
	}

	cmdID := c.Param("id")
	cmd, err := s.agentService.GetCommand(cmdID, agent.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load command"})
		return
	}

	var payload agentCommandReportPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	switch payload.Status {
	case "completed", "error":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	now := time.Now()
	cmd.Status = payload.Status
	cmd.Result = payload.Result
	cmd.Error = payload.Error
	cmd.CompletedAt = &now

	if payload.Status == "error" && cmd.Type == "check-update" {
		containerID := ""
		if v, ok := cmd.Payload["containerId"].(string); ok {
			containerID = v
		}
		if err := s.markAgentContainerError(agent, containerID, payload.Error); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if payload.Status == "completed" && payload.Result != nil {
		if err := s.applyCommandResult(agent, *cmd, payload.Result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	if cmd.Type == "update-container" || cmd.Type == "rollback-container" {
		containerID, name, image := containerDetailsFromReport(agent, payload.Result, cmd.Payload)
		if containerID != "" || name != "" {
			status := payload.Status
			message := payload.Error
			if status == "completed" {
				status = "success"
				if message == "" {
					if cmd.Type == "rollback-container" {
						message = "Rollback completed"
					} else {
						message = "Update completed"
					}
				}
			}
			s.recordUpdateHistory(UpdateHistory{
				ContainerID:   containerID,
				ContainerName: name,
				Image:         image,
				AgentID:       agent.ID,
				AgentName:     agent.Name,
				Source:        "agent",
				Status:        status,
				Message:       message,
			})
		}
	}

	if err := s.agentService.UpdateCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "command result recorded"})
}

func (s *Server) toggleAgentContainerAutoUpdate(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}

	agentID := c.Param("id")
	containerID := c.Param("containerId")
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := s.agentService.ToggleContainerAutoUpdate(agentID, containerID, payload.Enabled); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update container auto-update setting"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auto-update preference saved"})
}

func (s *Server) applyCommandResult(agent *Agent, cmd AgentCommand, res JSONMap) error {
	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
	updated := false

	switch cmd.Type {
	case "check-update":
		containerID, _ := res["containerId"].(string)
		if containerID == "" {
			return errors.New("missing containerId in result")
		}
		updateAvailable, _ := res["updateAvailable"].(bool)
		now := time.Now()
		for i, cont := range agent.Containers {
			if cont.ID == containerID {
				agent.Containers[i].UpdateAvailable = updateAvailable
				agent.Containers[i].CheckedAt = &now
				updated = true
				break
			}
		}
		if !updated {
			agent.Containers = append(agent.Containers, ContainerSnapshot{
				ID:              containerID,
				UpdateAvailable: updateAvailable,
				CheckedAt:       &now,
			})
		}
	case "update-container", "rollback-container":
		handled, err := s.applyContainerSnapshotResult(agent, res, cmd.Payload)
		if err != nil {
			return err
		}
		updated = handled
	}

	if updated {
		now := time.Now()
		agent.LastSeen = &now
		if err := silentDB.Save(agent).Error; err != nil {
			return fmt.Errorf("failed to update agent containers: %w", err)
		}
	}
	return nil
}

func (s *Server) applyContainerSnapshotResult(agent *Agent, res JSONMap, payload JSONMap) (bool, error) {
	var snapshot *ContainerSnapshot
	if raw, ok := res["container"].(map[string]interface{}); ok {
		parsed := toContainerSnapshot(raw)
		if prev, ok := res["autoUpdate"].(bool); ok {
			parsed.AutoUpdate = prev
		}
		parsed.UpdateAvailable = false
		snapshot = &parsed
	}
	containerID, _ := res["containerId"].(string)
	now := time.Now()
	if snapshot == nil && containerID != "" {
		for _, cont := range agent.Containers {
			if cont.ID == containerID {
				copy := cont
				copy.UpdateAvailable = false
				copy.CheckedAt = &now
				snapshot = &copy
				break
			}
		}
	}
	if containerID == "" && snapshot != nil {
		containerID = snapshot.ID
	}
	if containerID == "" {
		if payload != nil {
			if v, ok := payload["containerId"].(string); ok && v != "" {
				containerID = v
			}
		}
	}
	if containerID == "" {
		return false, errors.New("missing containerId in result")
	}
	replaced := false
	for i, cont := range agent.Containers {
		if cont.ID == containerID {
			if snapshot != nil {
				if cont.AutoUpdate && !snapshot.AutoUpdate {
					snapshot.AutoUpdate = true
				}
				agent.Containers[i] = *snapshot
			} else {
				agent.Containers[i].UpdateAvailable = false
				agent.Containers[i].CheckedAt = &now
			}
			replaced = true
			break
		}
	}
	if !replaced && snapshot != nil {
		agent.Containers = append(agent.Containers, *snapshot)
	}
	return true, nil
}

func (s *Server) markAgentContainerError(agent *Agent, containerID, message string) error {
	if containerID == "" {
		return errors.New("missing containerId in result")
	}

	now := time.Now()
	snapshot := ContainerSnapshot{
		ID:              containerID,
		State:           "error",
		Status:          message,
		UpdateAvailable: false,
		CheckedAt:       &now,
	}

	replaced := false
	for i, cont := range agent.Containers {
		if cont.ID == containerID {
			snapshot.Name = cont.Name
			snapshot.Image = cont.Image
			snapshot.AutoUpdate = cont.AutoUpdate
			agent.Containers[i] = snapshot
			replaced = true
			break
		}
	}
	if !replaced {
		agent.Containers = append(agent.Containers, snapshot)
	}

	agent.LastSeen = &now
	if err := s.db.Session(&gorm.Session{Logger: logger.Discard}).Save(agent).Error; err != nil {
		return fmt.Errorf("failed to update agent containers: %w", err)
	}
	return nil
}

func toContainerSnapshot(raw map[string]interface{}) ContainerSnapshot {
	cs := ContainerSnapshot{}
	if v, ok := raw["id"].(string); ok {
		cs.ID = v
	}
	if v, ok := raw["name"].(string); ok {
		cs.Name = v
	}
	if v, ok := raw["image"].(string); ok {
		cs.Image = v
	}
	if v, ok := raw["state"].(string); ok {
		cs.State = v
	}
	if v, ok := raw["status"].(string); ok {
		cs.Status = v
	}
	if v, ok := raw["autoUpdate"].(bool); ok {
		cs.AutoUpdate = v
	}
	if v, ok := raw["checkedAt"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			cs.CheckedAt = &t
		}
	}
	if v, ok := raw["updateAvailable"].(bool); ok {
		cs.UpdateAvailable = v
	}
	if arr, ok := raw["ports"].([]interface{}); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				cs.Ports = append(cs.Ports, s)
			}
		}
	}
	if arr, ok := raw["labels"].([]interface{}); ok {
		for _, item := range arr {
			if s, ok := item.(string); ok {
				cs.Labels = append(cs.Labels, s)
			}
		}
	}
	return cs
}
