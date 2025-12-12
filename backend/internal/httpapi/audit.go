package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) listAuditLogs(c *gin.Context) {
	limitParam := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitParam)

	logs, err := s.auditService.List(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, logs)
}
