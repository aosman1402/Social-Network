package middleware

// AllowedOrigins is the set of frontend origins permitted to make
// cross-origin requests to this API - both ordinary HTTP (server.go's CORS
// wrapper) and the WebSocket handshake (handlers.HandleWebSocket's
// upgrader), which isn't covered by regular CORS/preflight checks and has
// to validate Origin itself. Single source of truth - this list previously
// existed as two separately-maintained copies.
var AllowedOrigins = map[string]bool{
	"http://localhost:3000": true,
	"http://localhost:3001": true,
	"http://127.0.0.1:3000": true,
	"http://127.0.0.1:3001": true,
}
