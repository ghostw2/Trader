package strategy_test

import (
	"testing"

	"github.com/menribardhi/trader/internal/strategy"
)

func TestNoSignalBeforeWindow(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// 49 prices = one short of slowPeriod; all must be HOLD
	for i := 0; i < 49; i++ {
		eng.ProcessPrice(100.0)
		sig := <-ch
		if sig.Side != "HOLD" {
			t.Fatalf("price %d: expected HOLD before window full, got %s", i+1, sig.Side)
		}
	}
}

func TestBuySignalOnCrossoverUp(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// Fill window with 50 flat prices at 10 -> fastPrev=slowPrev=10
	for i := 0; i < 50; i++ {
		eng.ProcessPrice(10.0)
		<-ch // drain
	}
	// Add price at 200:
	//   window=[10x49, 200]; SMA(10)=(10x9+200)/10=29; SMA(50)=(10x49+200)/50=13.8
	//   fastPrev(10) <= slowPrev(10) && fastNow(29) > slowNow(13.8) -> BUY
	eng.ProcessPrice(200.0)
	sig := <-ch
	if sig.Side != "BUY" {
		t.Errorf("expected BUY on crossover up, got %s (SMAFast=%v SMASlow=%v)", sig.Side, sig.SMAFast, sig.SMASlow)
	}
}

func TestSellSignalOnCrossoverDown(t *testing.T) {
	eng := strategy.NewEngine(nil)
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

	// Fill window with 50 flat prices at 200 -> fastPrev=slowPrev=200
	for i := 0; i < 50; i++ {
		eng.ProcessPrice(200.0)
		<-ch // drain
	}
	// Add price at 10:
	//   window=[200x49, 10]; SMA(10)=(200x9+10)/10=181; SMA(50)=(200x49+10)/50=198.2
	//   fastPrev(200) >= slowPrev(200) && fastNow(181) < slowNow(198.2) -> SELL
	eng.ProcessPrice(10.0)
	sig := <-ch
	if sig.Side != "SELL" {
		t.Errorf("expected SELL on crossover down, got %s (SMAFast=%v SMASlow=%v)", sig.Side, sig.SMAFast, sig.SMASlow)
	}
}
