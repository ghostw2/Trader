package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
)

// Server is the HTTP server. It implements http.Handler.
type Server struct {
	hub    *hub.Hub
	router chi.Router
}

// New creates a Server wired to the given hub.
func New(h *hub.Hub) *Server {
	s := &Server{hub: h}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
