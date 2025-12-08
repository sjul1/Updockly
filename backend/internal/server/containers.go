package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type containerResponse struct {
	ID              string   `json:"ID"`
	Name            string   `json:"Name"`
	Image           string   `json:"Image"`
	State           string   `json:"State"`
	Status          string   `json:"Status"`
	AutoUpdate      bool     `json:"AutoUpdate"`
	UpdateAvailable bool     `json:"UpdateAvailable"`
	Ports           []string `json:"Ports"`
}

func (s *Server) listContainers(c *gin.Context) {
	ctx := c.Request.Context()
	containers, err := s.containerService.ListContainers(ctx)
	if err != nil {
		log.Printf("listContainers: failed to list containers: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list containers: %v", err)})
		return
	}

	resp := make([]containerResponse, 0, len(containers))
	for _, cont := range containers {
		resp = append(resp, containerResponse{
			ID:              cont.ID,
			Name:            cont.Name,
			Image:           cont.Image,
			State:           cont.State,
			Status:          cont.Status,
			AutoUpdate:      cont.AutoUpdate,
			UpdateAvailable: cont.UpdateAvailable,
			Ports:           cont.Ports,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) localHostInfo(c *gin.Context) {
	info, err := s.containerService.GetHostInfo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get host info"})
		return
	}
	c.JSON(http.StatusOK, info)
}

func (s *Server) checkContainerUpdateHandler(c *gin.Context) {
	id := c.Param("id")
	ok, err := s.containerService.CheckUpdate(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"updateAvailable": ok})
}

func (s *Server) toggleAutoUpdateHandler(c *gin.Context) {
	id := c.Param("id")
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := s.containerService.ToggleAutoUpdate(c.Request.Context(), id, payload.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist container settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auto-update preference updated"})
}

func (s *Server) startContainerHandler(c *gin.Context) {
	id := c.Param("id")
	if err := s.containerService.StartContainer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to start container: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container started"})
}

func (s *Server) stopContainerHandler(c *gin.Context) {
	id := c.Param("id")
	if err := s.containerService.StopContainer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to stop container: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container stopped"})
}

func (s *Server) restartContainerHandler(c *gin.Context) {
	id := c.Param("id")
	if err := s.containerService.RestartContainer(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to restart container: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Container restarted"})
}

func (s *Server) containerLogsHandler(c *gin.Context) {
	id := c.Param("id")
	tail := c.DefaultQuery("tail", "200")
	logs, err := s.containerService.GetLogs(c.Request.Context(), id, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to fetch logs: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (s *Server) countAutoUpdateContainers(c *gin.Context) {
	count, err := s.containerService.CountAutoUpdate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count auto-update containers"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (s *Server) updateContainerHandler(c *gin.Context) {
	id := c.Param("id")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	c.Header("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(c.Writer)
	send := func(payload map[string]interface{}) {
		_ = encoder.Encode(payload)
		flusher.Flush()
	}

	newID, name, image, digest, err := s.containerService.UpdateContainer(c.Request.Context(), id, send)
	if err != nil {
		rolledBack := false
		status := "error"
		rollbackMsg := ""
		if ue := new(UpdateError); errors.As(err, &ue) {
			rolledBack = ue.RolledBack
			if rolledBack {
				status = "warning"
				rollbackMsg = ue.RollbackMessage
			}
		}
		msg := err.Error()
		s.recordUpdateHistory(UpdateHistory{
			ContainerID:   id,
			ContainerName: name,
			Image:         image,
			ImageDigest:   digest,
			Source:        "manual",
			Status:        status,
			Message:       msg,
		})
		payload := map[string]interface{}{
			"error":      msg,
			"rolledBack": rolledBack,
		}
		if rolledBack {
			msgText := rollbackMsg
			if strings.TrimSpace(msgText) == "" {
				msgText = "Update failed but the previous container was restored."
			}
			payload["rollbackMessage"] = msgText
		}
		send(payload)
		return
	}

	s.recordUpdateHistory(UpdateHistory{
		ContainerID:   newID,
		ContainerName: name,
		Image:         image,
		ImageDigest:   digest,
		Source:        "manual",
		Status:        "success",
		Message:       "Update completed",
	})

	send(map[string]interface{}{
		"message": fmt.Sprintf("Container %s updated successfully", name),
		"newId":   newID,
	})
}

func (s *Server) rollbackContainerHandler(c *gin.Context) {
	id := c.Param("id")
	var payload struct {
		Image     string `json:"image"`
		HistoryID string `json:"historyId,omitempty"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil || strings.TrimSpace(payload.Image) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	targetImage := strings.TrimSpace(payload.Image)

	name, newID, err := s.containerService.RollbackContainer(c.Request.Context(), id, targetImage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Rolled back %s to %s", name, targetImage),
		"newId":   newID,
	})
}
