package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"updockly/backend/internal/config"
)

func TestSendWebhookMessage(t *testing.T) {
	var receivedBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	})
	// Force IPv4 listener to avoid environments that disallow IPv6
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("unable to open test listener: %v", err)
	}
	ts := httptest.NewUnstartedServer(handler)
	ts.Listener = l
	ts.Start()
	defer ts.Close()

	srv := &Server{
		cfg: config.Config{
			Notifications: config.NotificationSettings{
				WebhookURL: ts.URL,
			},
		},
	}

	err = srv.sendWebhookMessage(context.Background(), "Test content")
	if err != nil {
		t.Fatalf("sendWebhookMessage failed: %v", err)
	}

	var msg discordMessage
	if err := json.Unmarshal(receivedBody, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Test content" {
		t.Errorf("expected content 'Test content', got %q", msg.Content)
	}
}
