package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/binance"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticks := make(chan models.Tick, 64)

	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)

	go client.Run(ctx)
	go h.Run(ctx)

	srv := api.New(h)

	log.Info().Msg("trader listening on :8080")
	if err := http.ListenAndServe(":8080", srv); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
