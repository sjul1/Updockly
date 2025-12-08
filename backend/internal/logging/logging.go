package logging

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// New builds a JSON slog logger with the provided level (info by default).
func New(level string) *slog.Logger {
	lvl := parseLevel(level)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:       lvl,
		AddSource:   false,
		ReplaceAttr: nil,
	})
	return slog.New(handler)
}

func parseLevel(val string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type ctxKey struct{}

// FromContext returns a logger embedded in the Gin context or the default slog logger.
func FromContext(c *gin.Context) *slog.Logger {
	if c == nil {
		return slog.Default()
	}
	if val, ok := c.Get(loggerKey()); ok {
		if lgr, ok := val.(*slog.Logger); ok && lgr != nil {
			return lgr
		}
	}
	return slog.Default()
}

// Middleware injects a request-scoped logger and emits structured access logs.
func Middleware(base *slog.Logger) gin.HandlerFunc {
	if base == nil {
		base = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		reqID := c.GetHeader("X-Request-Id")
		if reqID == "" {
			reqID = uuid.NewString()
		}

		reqLogger := base.With(
			"req_id", reqID,
			"method", c.Request.Method,
			"path", c.FullPath(),
			"client_ip", c.ClientIP(),
		)

		c.Set(loggerKey(), reqLogger)
		c.Header("X-Request-Id", reqID)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		errStr := ""
		if len(c.Errors) > 0 {
			errStr = c.Errors.ByType(gin.ErrorTypePrivate).String()
		}

		reqLogger.Info("request.complete",
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"bytes_out", c.Writer.Size(),
			"error", errStr,
		)
	}
}

func loggerKey() string { return "logger" }
