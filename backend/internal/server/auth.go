package server

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing authorization header"})
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid authorization header"})
			return
		}
		claims, err := s.authService.VerifyToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}

func getClaims(c *gin.Context) *TokenClaims {
	value, ok := c.Get("claims")
	if !ok {
		return nil
	}
	claims, _ := value.(*TokenClaims)
	return claims
}