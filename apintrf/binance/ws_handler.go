package binance

import (
	"fmt"
	"strings"
	"sync"
)

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

var buyMarket orderPrice
var sellMarket orderPrice

func resp_hander(resp *WsStream, markets TrdMarkets) {
	//TODO change trim to split after @ and take first slice
	markt := strings.TrimSuffix(resp.Stream, "@depth5")
	switch markt {
	case markets.BuyMarket:
		price, _ := resp.Data.Asks[2][0].Float64()
		buyMarket.update_price(price)
	case markets.SellMarket:
		fPrice, _ := resp.Data.Bids[2][0].Float64()
		price := fx_conv(fPrice)
		sellMarket.update_price(price)
	}
}

// TODO use global var or not?
var avgFxRate = 1.0

func fx_conv(price float64) float64 {
	return price / avgFxRate
}

// TODO use global var or not?
var avgSpread = 2.0

func spread_calc(baseP, quoteP float64) bool {
	if quoteP-baseP >= avgSpread {
		return true
	} else {
		return false
	}
}
