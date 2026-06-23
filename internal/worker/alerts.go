package worker

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	dbpkg "github.com/menribardhi/trader/internal/db"
	"github.com/menribardhi/trader/internal/hub"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog/log"
)

type AlertChecker struct {
	hub *hub.Hub
	db  *sql.DB
}

func NewAlertChecker(h *hub.Hub, sqldb *sql.DB) *AlertChecker {
	return &AlertChecker{hub: h, db: sqldb}
}

func (ac *AlertChecker) Run(ctx context.Context) {
	sub := ac.hub.Subscribe()
	defer ac.hub.Unsubscribe(sub)
	for {
		select {
		case tick, ok := <-sub:
			if !ok {
				return
			}
			ac.check(tick)
		case <-ctx.Done():
			return
		}
	}
}

func (ac *AlertChecker) check(tick models.Tick) {
	price, err := strconv.ParseFloat(tick.Price, 64)
	if err != nil {
		return
	}
	alerts, err := dbpkg.ListActiveAlerts(ac.db)
	if err != nil {
		return
	}
	now := time.Now().UnixMilli()
	for _, a := range alerts {
		hit := (a.Direction == "above" && price >= a.TargetPrice) ||
			(a.Direction == "below" && price <= a.TargetPrice)
		if !hit {
			continue
		}
		if err := dbpkg.MarkTriggered(ac.db, a.ID, now); err != nil {
			log.Error().Err(err).Int64("id", a.ID).Msg("worker: mark triggered failed")
			continue
		}
		log.Info().
			Str("symbol", tick.Symbol).
			Float64("price", price).
			Float64("target", a.TargetPrice).
			Str("direction", a.Direction).
			Msg("alert triggered")
	}
}
