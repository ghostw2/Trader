package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/portfolio"
)

type Server struct {
	hub    *hub.Hub
	db     *sql.DB
	feed   *portfolio.PriceFeed
	router chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed) *Server {
	s := &Server{hub: h, db: sqldb, feed: feed}
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/ws", s.handleWS)
	r.Route("/api", func(r chi.Router) {
		r.Post("/alerts", s.handleCreateAlert)
		r.Get("/alerts", s.handleListAlerts)
		r.Delete("/alerts/{id}", s.handleDeleteAlert)
		r.Get("/portfolio", s.handleGetPortfolio)
		r.Post("/orders", s.handleCreateOrder)
		r.Get("/trades", s.handleListTrades)
	})
	s.router = r
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
