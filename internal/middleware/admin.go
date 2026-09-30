package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireAdmin validates the X-Api-Key header against the configured API key.
func RequireAdmin(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" || r.Header.Get("X-Api-Key") != apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "unauthorized", "code": "UNAUTHORIZED",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
