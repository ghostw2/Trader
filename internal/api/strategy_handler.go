package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/menribardhi/trader/internal/binance"
	"github.com/menribardhi/trader/internal/strategy"
)

func (s *Server) handleGetSignals(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.eng.Subscribe()
	defer s.eng.Unsubscribe(ch)

	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(sig)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleRunBacktest(w http.ResponseWriter, r *http.Request) {
	if s.eng == nil {
		http.Error(w, "not configured", http.StatusServiceUnavailable)
		return
	}
	closes, err := binance.FetchKlinesFromURL(s.klinesURL)
	if err != nil {
		http.Error(w, "failed to fetch historical data", http.StatusServiceUnavailable)
		return
	}
	result := strategy.NewBacktester().Run(closes)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
