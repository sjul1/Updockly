package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) runningHistoryHandler(c *gin.Context) {
	rows, err := s.metricsService.RunningHistory(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}
