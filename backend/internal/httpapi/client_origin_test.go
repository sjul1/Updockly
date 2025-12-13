package httpapi

import "testing"

func TestAbsoluteClientURL_PicksFirstOrigin(t *testing.T) {
	s := &Server{}
	s.cfg.ClientOrigin = "http://a.example, https://b.example"

	if got := s.absoluteClientURL("/auth/callback"); got != "http://a.example/auth/callback" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestAbsoluteClientURL_EmptyOriginFallsBackToPath(t *testing.T) {
	s := &Server{}
	s.cfg.ClientOrigin = ""

	if got := s.absoluteClientURL("auth/callback"); got != "/auth/callback" {
		t.Fatalf("unexpected url: %s", got)
	}
}

