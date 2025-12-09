package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) listUpdateHistory(c *gin.Context) {
	rows, err := s.historyService.List(c.Query("limit"))
	if err != nil {
		if err.Error() == "database not ready" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (s *Server) deleteUpdateHistory(c *gin.Context) {
	err := s.historyService.Delete(c.Param("id"))
	if err == nil {
		c.Status(http.StatusNoContent)
		return
	}
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "history entry not found"})
		return
	}
	if err != nil && err.Error() == "database not ready" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func (s *Server) recordUpdateHistory(entry UpdateHistory) {
	recorded, err := s.historyService.Record(entry)
	if err != nil {
		return
	}
	go s.sendImmediateNotification(recorded)
}
