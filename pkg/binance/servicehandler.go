package binance

import (
	"context"
	"time"
)

var timeoutCh = make(chan time.Duration)

func service_handler(ctx context.Context) {
	//Connect to WS and send req to start data stream
	wsConn := connect_ws()
	subscribeStream(wsConn, trdStrategy.BuyMarket, trdStrategy.SellMarket, trdStrategy.ConvMarket)
	//Create a ticker to disconnect and the reconnect the WS after 24h. Becasue Binance will drop the connection after 24h we will prevent this from causing an unexpected error by doing it manually.
	d, _ := time.ParseDuration("23h55m")
	wsTicker := time.Tick(d)
	//Create a copy of the parent ctx to create new child ctx when needed
	parentCtx := ctx
	//Ctx for trd_handler func
	var ctxTrdCncl context.CancelFunc
	ctxTrd, ctxTrdCncl = context.WithCancel(parentCtx)
	//Ctx for Ws listen func
	var ctxWsCncl context.CancelFunc
	ctxWs, ctxWsCncl = context.WithCancel(parentCtx)

	//Start services
	go rLimits_handler(exLimitsCtrs)
	go listen_ws(ws)
	go trd_handler(ctxTrd)

	restartCh := make(chan bool)
	for {
		select {
		case d := <-timeoutCh:
			ctxTrdCncl()
			go timeout_reqs(restartCh, d)
			break
		case <-restartCh:
			ctx, ctxTrdCncl = context.WithCancel(parentCtx)
			go trd_handler(ctx)
			break
		case <-wsTicker:
			//First cancel the ctx and then close connection to prevent from reading old data.
			ctxWsCncl()
			wsConn.Close()
			//Reconnect to the ws and resubscribe to the data stream. Then restart the listen service.
			wsConn = connect_ws()
			subscribeStream(wsConn, trdStrategy.BuyMarket, trdStrategy.SellMarket, trdStrategy.ConvMarket)
			go listen_ws()
			break
		}
	}
}

func timeout_reqs(ch chan<- bool, d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ch <- true
			return
		}
	}
}
