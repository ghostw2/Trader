package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/menribardhi/trader/internal/api"
	"github.com/menribardhi/trader/internal/binance"
	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/menribardhi/trader/internal/portfolio"
	"github.com/menribardhi/trader/internal/strategy"
	"github.com/menribardhi/trader/internal/worker"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sqldb, err := dbpkg.Open("./trader.db")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open database")
	}
	defer sqldb.Close()

	if err := dbpkg.InitPortfolio(sqldb); err != nil {
		log.Fatal().Err(err).Msg("failed to init portfolio")
	}

	ticks := make(chan models.Tick, 64)
	h := hub.New(ticks)
	client := binance.New("BTCUSDT", ticks)
	feed := portfolio.NewPriceFeed(h)
	eng := strategy.NewEngine(h)

	go client.Run(ctx)
	go h.Run(ctx)
	go feed.Run(ctx)
	go eng.Run(ctx)
	go worker.NewAlertChecker(h, sqldb).Run(ctx)

	httpSrv := &http.Server{Addr: ":8080", Handler: api.New(h, sqldb, feed, eng)}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Shutdown(context.Background())
	}()
	log.Info().Msg("trader listening on :8080")
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}
