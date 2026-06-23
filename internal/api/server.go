package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
)

type Server struct {
	hub    *hub.Hub
	db     *sql.DB
	router chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB) *Server {
	s := &Server{hub: h, db: sqldb}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	r.Route("/api", func(r chi.Router) {
		r.Post("/alerts", s.handleCreateAlert)
		r.Get("/alerts", s.handleListAlerts)
		r.Delete("/alerts/{id}", s.handleDeleteAlert)
	})
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
