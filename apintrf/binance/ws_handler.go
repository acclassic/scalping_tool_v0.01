package binance

import (
	"context"
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

//TODO replace markets with global var
func resp_hander(ctx context.Context, resp *WsStream) {
	//If resp empty ignore, means sendet req was sucessfull
	//if len(resp.Data.Asks) <= 0 && len(resp.Data.Bids) <= 0 {
	//	return
	//}

	////TODO change trim to split after @ and take first slice
	//markt := strings.TrimSuffix(resp.Stream, "@depth5@100ms")
	//switch strings.ToUpper(markt) {
	//case trdStrategy.BuyMarket:
	//	price, _ := resp.Data.Asks[0][0].Float64()
	//	//price, _ := resp.Data.Asks[1][0].Float64()
	//	buyMarketP.update_price(price)
	//case trdStrategy.SellMarket:
	//	price, _ := resp.Data.Bids[0][0].Float64()
	//	//price, _ := resp.Data.Bids[1][0].Float64()
	//	sellMarketP.update_price(price)
	//case trdStrategy.ConvMarket:
	//	price, _ := resp.Data.Asks[0][0].Float64()
	//	//price, _ := resp.Data.Asks[2][0].Float64()
	//	convMarketP.update_price(price)
	//}
	//TODO check with sleeper and witohout goroutine
	//for {
	//	select {
	//	case <-ctx.Done():
	//		fmt.Println("return")
	//		return
	//	default:
	//		fmt.Println(time.UnixMilli(resp.Data.S))
	//	}
	//}
	//DEL
	//params := queryParams{
	//	"symbol": "EURUSDT",
	//}
	//reqParams := httpReq{
	//	method:  "GET",
	//	url:     "/api/v3/avgPrice",
	//	qParams: params,
	//}
	//http_req_handler(ctx, reqParams)
	//DEL
}
