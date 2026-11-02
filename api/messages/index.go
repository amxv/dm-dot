package handler

import (
	"net/http"

	"github.com/amxv/dm-dot/pkg/server"
)

// Handler exposes the authenticated, one-way REST message ingress.
func Handler(w http.ResponseWriter, r *http.Request) {
	server.RESTHandler().ServeHTTP(w, r)
}
