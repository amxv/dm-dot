package handler

import (
	"net/http"

	"github.com/amxv/dm-dot/pkg/server"
)

// Handler exposes a stateless MCP endpoint as a Vercel Go Function.
func Handler(w http.ResponseWriter, r *http.Request) {
	server.Handler().ServeHTTP(w, r)
}
