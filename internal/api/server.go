package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/portfolio"
	"github.com/menribardhi/trader/internal/strategy"
)

const defaultKlinesURL = "https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=5m&limit=8640"

type Server struct {
	hub       *hub.Hub
	db        *sql.DB
	feed      *portfolio.PriceFeed
	eng       *strategy.Engine
	klinesURL string
	router    chi.Router
}

func New(h *hub.Hub, sqldb *sql.DB, feed *portfolio.PriceFeed, eng *strategy.Engine) *Server {
	s := &Server{
		hub:       h,
		db:        sqldb,
		feed:      feed,
		eng:       eng,
		klinesURL: defaultKlinesURL,
	}
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
		r.Get("/strategy/signals", s.handleGetSignals)
		r.Post("/strategy/backtest", s.handleRunBacktest)
	})
	s.router = r
	return s
}

// SetKlinesURL overrides the Binance klines URL. Used in tests to inject a mock server.
func (s *Server) SetKlinesURL(url string) {
	s.klinesURL = url
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
