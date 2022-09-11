package binance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var (
	buyMarketP  marketPrice
	sellMarketP marketPrice
	convMarketP marketPrice
)

var stratStatus = stratCtrl{connTimeout: false}

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

type stratCtrl struct {
	connTimeout bool
	cancelCh    chan time.Duration
	mu          sync.RWMutex
}

func (ctrl *stratCtrl) init() {
	ctrl.cancelCh = make(chan time.Duration)
}

func (ctrl *stratCtrl) conn_timeout() bool {
	ctrl.mu.RLock()
	defer ctrl.mu.RUnlock()
	return ctrl.connTimeout
}

func (ctrl *stratCtrl) update_status(b bool) {
	ctrl.mu.Lock()
	ctrl.connTimeout = b
	ctrl.mu.Unlock()
}

//TODO replace markets with global var
func resp_hander(ctx context.Context, resp *WsStream) {
	for {
		select {
		case interval := <-stratStatus.cancelCh:
			fmt.Println(interval)
			if stratStatus.conn_timeout() == false {
				stratStatus.update_status(true)
				go timeout_conns(interval)
			}
			return
		default:
			if stratStatus.conn_timeout() == false {
				fmt.Println(time.UnixMilli(resp.Data.S))
				//DEL
				params := queryParams{
					"symbol": "EURUSDT",
				}
				reqParams := httpReq{
					method:  "GET",
					url:     "/api/v3/avgPrice",
					qParams: params,
				}
				http_req_handler(ctx, reqParams)
				//DEL
				return
			} else {
				return
			}
		}
	}
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

	//if trd_signal() == true {
	//	fmt.Println("true")
	//} else {
	//	//If trd_signal false drop resp
	//	fmt.Println("false")
	//}
}

func trd_signal() bool {
	//TODO evtl. include sync.Waitgroup and goroutine
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

func timeout_conns(d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			stratStatus.update_status(false)
			return
		}
	}
}
