package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"updockly/backend/internal/logging"
)

// respondError logs the error with request context and returns a safe client-facing message.
func respondError(c *gin.Context, status int, clientMsg string, err error) {
	if err != nil {
		logging.FromContext(c).Error(clientMsg,
			"status", status,
			"error", err,
			"path", c.FullPath(),
		)
	} else {
		logging.FromContext(c).Warn(clientMsg, "status", status, "path", c.FullPath())
	}
	c.JSON(status, gin.H{"error": clientMsg})
}

func wrapErr(msg string, err error) error {
	if err == nil {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// respondInternal simplifies internal server errors.
func respondInternal(c *gin.Context, msg string, err error) {
	respondError(c, http.StatusInternalServerError, msg, err)
}
