package binance

import (
	"fmt"
	"sync"
	"time"
)

var buyMarketP orderPrice
var sellMarketP orderPrice

type orderPrice struct {
	mu    sync.Mutex
	price float64
}

//Update order price for market.
func (op *orderPrice) update_price(price float64) {
	op.mu.Lock()
	op.price = price
	op.mu.Unlock()
}

//TODO replace markets with global var
func resp_hander(resp *WsStream) {
	fmt.Println(respfmt.Sprintf("%s@depth10@100ms", trdStrategy.Market))
	//If resp empty ignore
	if len(resp.Data.Asks) <= 0 && len(resp.Data.Bids) <= 0 {
		return
	}
	//fmt.Println(resp)
	var askCache []float64
	var bidCache []float64
	for _, v := range resp.Data.Asks {
		f, _ := v[0].Float64()
		askCache = append(askCache, f)
	}
	for _, v := range resp.Data.Bids {
		f, _ := v[0].Float64()
		bidCache = append(bidCache, f)
	}
	fmt.Printf("Ask MA 5: %f\n", calc_ma(5, askCache))
	fmt.Printf("Ask MA 10: %f\n", calc_ma(10, askCache))
	fmt.Printf("Bid MA 5: %f\n", calc_ma(5, bidCache))
	fmt.Printf("Bid MA 10: %f\n", calc_ma(10, bidCache))
	//TODO change trim to split after @ and take first slice
	//markt := strings.TrimSuffix(resp.Stream, "@depth5")
	//switch markt {
	//case markets.BuyMarket:
	//	price, _ := resp.Data.Asks[2][0].Float64()
	//	buyMarketP.update_price(price)
	//case markets.SellMarket:
	//	fPrice, _ := resp.Data.Bids[2][0].Float64()
	//	price := fx_conv(fPrice)
	//	sellMarketP.update_price(price)
	//}
	//if spread_calc(buyMarketP.price, sellMarketP.price) {
	//	go buy_handler()
	//}
}

func calc_ma(maBase int, cache []float64) float64 {
	var ma float64
	for i := 0; i < maBase; i++ {
		ma += cache[i]
	}
	return ma / float64(maBase)
}

var buyCh = make(chan bool, 1)

func buy_handler() {
	buyCh <- true
	//get_funds("BTC")
	//get_avg_spread("BTCUSDT")
	time.Sleep(time.Minute)
	<-buyCh
}

// TODO use global var or not?
var avgFxRate = 37.4938

func fx_conv(price float64) float64 {
	return price / avgFxRate
}

// TODO use global var or not? Use avgSpread or just use safty net ex. fixed min. spread?
var avgSpread float64

func set_avgSpread(ticker *time.Ticker) {
	//for range ticker.C {
	//	bPrices := get_spread_prices(trdStrategy.BuyMarket)
	//	sPrices := get_spread_prices(trdStrategy.SellMarket)
	//	for k, v := range sPrices {
	//		sPrices[k] = fx_conv(v)
	//	}
	//	var spreads float64
	//	for i := 0; i < len(bPrices); i++ {
	//		spread := sPrices[i] - bPrices[i]
	//		spreads += spread
	//	}
	//	avgSpread = spreads / float64(trdStrategy.SpreadBase)
	//}
}

//TODO unquote block. For test reasons this func returns always true.
func spread_calc(baseP, quoteP float64) bool {
	return true
	//if quoteP-baseP >= avgSpread {
	//	return true
	//} else {
	//	return false
	//}
}
