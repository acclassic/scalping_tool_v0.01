package binance

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type orderPrice struct {
	mu    sync.Mutex
	price float64
}

//TODO implement method with mutex lock to update the MPrices
func (op *orderPrice) update_price(price float64) {
	op.mu.Lock()
	op.price = price
	op.mu.Unlock()
	fmt.Println(op.price)
}

var buyMarket orderPrice
var sellMarket orderPrice

func resp_hander(resp *WsStream, markets TrdMarkets) {
	fmt.Println(time.Now())
	fmt.Println(resp)
	//TODO change trim to split after @ and take first slice
	markt := strings.TrimSuffix(resp.Stream, "@depth5")
	switch markt {
	case markets.BuyMarket:
		//something
	case markets.SellMarket:
		//price, _ := resp.Data.Bids[2][0].Float64()
	}
}

type order struct {
	buyMPrice  float64
	sellMPrice float64
}

//type orderQueue [][]order
var orderQueue = make(chan order, 10)

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

func buy_handler(market [][]json.Number) {
}
