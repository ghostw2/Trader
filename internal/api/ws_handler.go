package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sub := s.hub.Subscribe()
	defer s.hub.Unsubscribe(sub)

	closed := make(chan struct{})
	// gorilla/websocket: one goroutine reads, one writes — contract satisfied.
	// conn.Close() unblocking ReadMessage is explicitly safe per gorilla docs.
	go func() {
		defer close(closed)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			data, err := json.Marshal(tick)
			if err != nil {
				log.Warn().Err(err).Msg("ws: failed to marshal tick")
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-closed:
			return
		}
	}
}
