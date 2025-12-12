package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

func (s *Server) csrfMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for agent endpoints (they use token auth)
		if strings.HasPrefix(c.Request.URL.Path, "/api/agents") {
			c.Next()
			return
		}

		// 1. Ensure CSRF cookie exists
		token, err := c.Cookie(csrfCookieName)
		if err != nil || token == "" {
			token = generateRandomString(32)
			setCsrfCookie(c, token, s.cfg.ClientOrigin)
		}

		// 2. Validate on state-changing methods
		if isUnsafeMethod(c.Request.Method) {
			headerToken := c.GetHeader(csrfHeaderName)
			if headerToken == "" || headerToken != token {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token mismatch"})
				return
			}
		}

		c.Next()
	}
}

func setCsrfCookie(c *gin.Context, token, clientOrigin string) {
	secure := false
	if origin := strings.Split(strings.TrimSpace(clientOrigin), ",")[0]; strings.HasPrefix(strings.ToLower(origin), "https://") {
		secure = true
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		// HttpOnly must be false so the frontend can read it and send it in the header
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
