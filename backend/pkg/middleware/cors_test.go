package middleware

import "testing"

// TestAllowedOrigins guards the CORS/WebSocket-origin allowlist against an
// accidental edit - both server.go's cors() wrapper and HandleWebSocket's
// upgrader key off this single map, so a typo'd entry here silently opens
// or breaks cross-origin access everywhere at once.
func TestAllowedOrigins(t *testing.T) {
	mustAllow := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:3001",
	}
	for _, origin := range mustAllow {
		if !AllowedOrigins[origin] {
			t.Errorf("AllowedOrigins[%q] = false, want true", origin)
		}
	}

	mustReject := []string{
		"http://evil.example.com",
		"https://localhost:3000", // different scheme
		"http://localhost:3002",  // different port
		"",
	}
	for _, origin := range mustReject {
		if AllowedOrigins[origin] {
			t.Errorf("AllowedOrigins[%q] = true, want false", origin)
		}
	}
}
