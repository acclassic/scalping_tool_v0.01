package binance

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var buyMarketP marketPrice
var sellMarketP marketPrice
var convMarketP marketPrice

type marketPrice struct {
	mu    sync.RWMutex
	price float64
}

//Update order price for markets.
func (mp *marketPrice) update_price(price float64) {
	mp.mu.Lock()
	mp.price = price
	mp.mu.Unlock()
}

//Read order prices for markets.
func (mp *marketPrice) get_price() float64 {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.price
}

//TODO replace markets with global var
func resp_hander(resp *WsStream) {
	//fmt.Println(resp)
	//If resp empty ignore, means sendet req was sucessfull
	if len(resp.Data.Asks) <= 0 && len(resp.Data.Bids) <= 0 {
		return
	}

	//TODO change trim to split after @ and take first slice
	markt := strings.TrimSuffix(resp.Stream, "@depth5@100ms")
	switch strings.ToUpper(markt) {
	case trdStrategy.BuyMarket:
		price, _ := resp.Data.Asks[0][0].Float64()
		//price, _ := resp.Data.Asks[1][0].Float64()
		buyMarketP.update_price(price)
	case trdStrategy.SellMarket:
		price, _ := resp.Data.Bids[0][0].Float64()
		//price, _ := resp.Data.Bids[1][0].Float64()
		sellMarketP.update_price(price)
	case trdStrategy.ConvMarket:
		price, _ := resp.Data.Asks[0][0].Float64()
		//price, _ := resp.Data.Asks[2][0].Float64()
		convMarketP.update_price(price)
	}
	if trd_signal() == true {
		fmt.Println("true")
	} else {
		fmt.Println("false")
	}
}

func trd_signal() bool {
	buyPrice := buyMarketP.get_price()
	sellPrice := sellMarketP.get_price()
	convPrice := convMarketP.get_price()
	if m := sellPrice/convPrice - buyPrice; m > 0 {
		fmt.Println(m)
		return true
	} else {
		fmt.Println(m)
		return false
	}
}

var buyCh = make(chan bool, 1)

func buy_handler() {
	buyCh <- true
	//get_funds("BTC")
	//get_avg_spread("BTCUSDT")
	time.Sleep(time.Minute)
	<-buyCh
}
