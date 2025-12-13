package httpapi

import "strings"

func (s *Server) primaryClientOrigin() string {
	raw := strings.TrimSpace(s.cfg.ClientOrigin)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func (s *Server) absoluteClientURL(path string) string {
	origin := strings.TrimSuffix(s.primaryClientOrigin(), "/")
	if origin == "" {
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + path
	}
	if path == "" {
		return origin
	}
	if strings.HasPrefix(path, "/") {
		return origin + path
	}
	return origin + "/" + path
}

