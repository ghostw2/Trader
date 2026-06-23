package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/menribardhi/trader/internal/models"
	"github.com/rs/zerolog/log"
)

const binanceStreamURL = "wss://stream.binance.com:9443/ws/%s@miniTicker"

type miniTickerMsg struct {
	Symbol string `json:"s"`
	Close  string `json:"c"`
	Time   int64  `json:"E"`
}

// Client connects to a WebSocket price stream and writes Ticks to out.
type Client struct {
	url string
	out chan<- models.Tick
}

// New builds a Client for the given symbol using the Binance stream URL.
func New(symbol string, out chan<- models.Tick) *Client {
	return &Client{
		url: fmt.Sprintf(binanceStreamURL, strings.ToLower(symbol)),
		out: out,
	}
}

// NewWithURL builds a Client with an explicit WebSocket URL (for testing).
func NewWithURL(url string, out chan<- models.Tick) *Client {
	return &Client{url: url, out: out}
}

// Run connects and reads ticks until ctx is cancelled.
// On error it reconnects with exponential backoff (0s first retry, then 1s doubling to 30s max).
func (c *Client) Run(ctx context.Context) {
	backoff := time.Duration(0)
	for {
		if err := c.connect(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			if backoff > 0 {
				log.Warn().Err(err).Msgf("binance: reconnecting in %s", backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
			}
			if backoff == 0 {
				backoff = time.Second
			} else if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 0
	}
}

func (c *Client) connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var m miniTickerMsg
		if err := json.Unmarshal(msg, &m); err != nil {
			continue
		}
		select {
		case c.out <- models.Tick{Symbol: m.Symbol, Price: m.Close, Timestamp: m.Time}:
		case <-ctx.Done():
			return nil
		}
	}
}
