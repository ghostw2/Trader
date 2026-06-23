package api

import (
	"encoding/json"
	"errors"
	"net/http"

	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/models"
)

func (s *Server) handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	cash, btc, avgBuyPrice, err := dbpkg.GetPortfolio(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	var currentPrice, totalValue, unrealizedPL float64
	if s.feed != nil {
		if price, ok := s.feed.Latest(); ok {
			currentPrice = price
			totalValue = cash + btc*price
			unrealizedPL = btc * (price - avgBuyPrice)
		} else {
			totalValue = cash
		}
	} else {
		totalValue = cash
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.PortfolioState{
		CashBalance:  cash,
		BTCBalance:   btc,
		AvgBuyPrice:  avgBuyPrice,
		CurrentPrice: currentPrice,
		TotalValue:   totalValue,
		UnrealizedPL: unrealizedPL,
	})
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	var req models.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		http.Error(w, `side must be "buy" or "sell"`, http.StatusBadRequest)
		return
	}
	if req.Quantity <= 0 {
		http.Error(w, "quantity must be positive", http.StatusBadRequest)
		return
	}
	if s.feed == nil {
		http.Error(w, "no price feed", http.StatusServiceUnavailable)
		return
	}
	price, ok := s.feed.Latest()
	if !ok {
		http.Error(w, "no price available yet", http.StatusServiceUnavailable)
		return
	}
	trade, err := dbpkg.ExecuteOrder(s.db, req.Side, req.Quantity, price)
	if err != nil {
		if errors.Is(err, dbpkg.ErrInsufficientFunds) {
			http.Error(w, "insufficient funds", http.StatusUnprocessableEntity)
			return
		}
		if errors.Is(err, dbpkg.ErrInsufficientBTC) {
			http.Error(w, "insufficient BTC", http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(trade)
}

func (s *Server) handleListTrades(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	trades, err := dbpkg.ListTrades(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trades)
}
