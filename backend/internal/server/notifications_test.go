package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"updockly/backend/internal/config"
)

func TestSendWebhookMessage(t *testing.T) {
	var receivedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	srv := &Server{
		cfg: config.Config{
			Notifications: config.NotificationSettings{
				WebhookURL: ts.URL,
			},
		},
	}

	err := srv.sendWebhookMessage(context.Background(), "Test content")
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
