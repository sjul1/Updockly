package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (s *Server) listUpdateHistory(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}

	limit := 200
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	rows := []UpdateHistory{}
	if err := s.db.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}

	c.JSON(http.StatusOK, rows)
}

func (s *Server) deleteUpdateHistory(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not ready"})
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing history id"})
		return
	}

	result := s.db.Delete(&UpdateHistory{}, "id = ?", id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete history entry"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "history entry not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) recordUpdateHistory(entry UpdateHistory) {
	if s.db == nil {
		return
	}

	entry.Source = strings.TrimSpace(strings.ToLower(entry.Source))
	if entry.Source == "" {
		entry.Source = "local"
	}
	entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
	if entry.Status == "" {
		entry.Status = "success"
	}
	entry.Message = strings.TrimSpace(entry.Message)

	silentDB := s.db.Session(&gorm.Session{Logger: logger.Discard})
	_ = silentDB.Create(&entry).Error

	go s.sendImmediateNotification(entry)
}
