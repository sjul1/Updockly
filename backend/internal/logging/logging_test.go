package logging

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewSetsLevel(t *testing.T) {
	logger := New("debug")
	if logger == nil {
		t.Fatal("expected logger, got nil")
	}
}

func TestMiddlewareInjectsLoggerAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))

	r := gin.New()
	r.Use(Middleware(base))
	r.GET("/ping", func(c *gin.Context) {
		// ensure logger available
		logger := FromContext(c)
		logger.Info("handler hit")
		c.String(200, "pong")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	logOut := output.String()
	if !strings.Contains(logOut, `"req_id"`) {
		t.Fatalf("expected request id in logs, got %s", logOut)
	}
	if !strings.Contains(logOut, "handler hit") {
		t.Fatalf("expected handler log in output, got %s", logOut)
	}
}
