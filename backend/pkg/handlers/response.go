package handlers

import (
	"encoding/json"
	"net/http"
)

// writeJSONError writes a JSON error body ({"error": message}) with the
// given status code. Every handler uses this instead of the stdlib's
// http.Error, which writes a plain-text body - the frontend's apiFetch
// always calls response.json() on every response, success or failure, so a
// plain-text error body fails that parse and silently becomes `data: null`,
// which then crashes any caller doing `data.error || "fallback"` on a null
// with a TypeError instead of showing the intended fallback message.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
