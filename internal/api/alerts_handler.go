package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	dbpkg "github.com/menribardhi/trader/internal/db"
)

type createAlertRequest struct {
	Symbol      string  `json:"symbol"`
	TargetPrice float64 `json:"target_price"`
	Direction   string  `json:"direction"`
}

func (s *Server) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var req createAlertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Direction != "above" && req.Direction != "below" {
		http.Error(w, `direction must be "above" or "below"`, http.StatusBadRequest)
		return
	}
	if req.Symbol == "" || req.TargetPrice <= 0 {
		http.Error(w, "symbol and positive target_price required", http.StatusBadRequest)
		return
	}
	alert, err := dbpkg.CreateAlert(s.db, req.Symbol, req.Direction, req.TargetPrice)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(alert)
}

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := dbpkg.ListAlerts(s.db)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (s *Server) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := dbpkg.DeleteAlert(s.db, id); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
