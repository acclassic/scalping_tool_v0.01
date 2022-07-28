package binance

import (
	"fmt"
	"strings"
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
	fmt.Println(op.price)
}

func resp_hander(resp *WsStream, markets TrdMarkets) {
	//TODO change trim to split after @ and take first slice
	markt := strings.TrimSuffix(resp.Stream, "@depth5")
	switch markt {
	case markets.BuyMarket:
		price, _ := resp.Data.Asks[2][0].Float64()
		buyMarketP.update_price(price)
	case markets.SellMarket:
		fPrice, _ := resp.Data.Bids[2][0].Float64()
		price := fx_conv(fPrice)
		sellMarketP.update_price(price)
	}
	if spread_calc(buyMarketP.price, sellMarketP.price) {
		go buy_handler()
	}
}

var buyCh = make(chan bool, 1)

func buy_handler() {
	buyCh <- true
	get_funds("BTC")
	time.Sleep(time.Minute)
	<-buyCh
}

// TODO use global var or not?
var avgFxRate = 1.0

func fx_conv(price float64) float64 {
	return price / avgFxRate
}

// TODO use global var or not?
var avgSpread = 2.0

//TODO unquote block. For test reasons this func returns always true.
func spread_calc(baseP, quoteP float64) bool {
	return true
	//if quoteP-baseP >= avgSpread {
	//	return true
	//} else {
	//	return false
	//}
}
