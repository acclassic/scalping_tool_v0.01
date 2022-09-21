package binance

import (
	"context"
	"strings"
	"sync"
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
func (mp *marketPrice) get_price(wg *sync.WaitGroup) float64 {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	defer wg.Done()
	return mp.price
}

func resp_hander(ctx context.Context, resp *WsStream) {
	//TODO check of errors are handled and how a disconect is handled.
	//If resp empty ignore, means sendet req was sucessfull
	if len(resp.Data.Asks) <= 0 && len(resp.Data.Bids) <= 0 {
		return
	}

	////TODO change trim to split after @ and take first slice
	markt := strings.TrimSuffix(resp.Stream, "@depth5@100ms")
	switch strings.ToUpper(markt) {
	case trdStrategy.BuyMarket:
		price, _ := resp.Data.Asks[0][0].Float64()
		buyMarketP.update_price(price)
	case trdStrategy.SellMarket:
		price, _ := resp.Data.Bids[0][0].Float64()
		sellMarketP.update_price(price)
	case trdStrategy.ConvMarket:
		price, _ := resp.Data.Asks[0][0].Float64()
		convMarketP.update_price(price)
	}
}
