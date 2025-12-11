package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRespondErrorWritesJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test", nil)
	c.Params = append(c.Params, gin.Param{Key: "path", Value: "/test"})

	respondError(c, http.StatusBadRequest, "boom", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if got := w.Body.String(); got == "" || got == "{}" {
		t.Fatalf("expected JSON body, got %q", got)
	}
}

func TestWrapErr(t *testing.T) {
	if err := wrapErr("msg", nil); err == nil || err.Error() != "msg" {
		t.Fatalf("wrapErr should wrap message when err is nil")
	}
	orig := wrapErr("inner", nil)
	err := wrapErr("outer", orig)
	if err == nil || err.Error() != "outer: inner" {
		t.Fatalf("wrapErr did not chain properly: %v", err)
	}
}
