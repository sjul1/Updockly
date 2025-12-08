package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type schedulePayload struct {
	Name           string `json:"name"`
	CronExpression string `json:"cronExpression"`
}

func (s *Server) listSchedules(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database connection not available"})
		return
	}
	var schedules []Schedule
	if err := s.db.Order("created_at desc").Find(&schedules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load schedules"})
		return
	}
	c.JSON(http.StatusOK, schedules)
}

func (s *Server) createSchedule(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database connection not available"})
		return
	}

	var payload schedulePayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Name == "" || payload.CronExpression == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	schedule := Schedule{
		Name:           payload.Name,
		CronExpression: payload.CronExpression,
	}
	if err := s.db.Create(&schedule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create schedule"})
		return
	}
	c.JSON(http.StatusCreated, schedule)
}

func (s *Server) updateScheduleHandler(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database connection not available"})
		return
	}

	id := c.Param("id")
	var payload schedulePayload
	if err := c.ShouldBindJSON(&payload); err != nil || payload.Name == "" || payload.CronExpression == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	var schedule Schedule
	if err := s.db.First(&schedule, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}

	schedule.Name = payload.Name
	schedule.CronExpression = payload.CronExpression
	if err := s.db.Save(&schedule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update schedule"})
		return
	}
	c.JSON(http.StatusOK, schedule)
}
