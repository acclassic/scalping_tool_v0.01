package binance

import (
	"context"
	"time"

	"golang.org/x/net/websocket"
)

var timeoutCh = make(chan time.Duration)

func service_handler(ctx context.Context, ws *websocket.Conn) {
	parentCtx := ctx
	var ctxCancel context.CancelFunc
	ctx, ctxCancel = context.WithCancel(parentCtx)
	//Start services
	go rLimits_handler(exLimitsCtrs)
	go listen_ws(ws)
	go trd_handler(ctx)

	restartCh := make(chan bool)
	for {
		select {
		case d := <-timeoutCh:
			ctxCancel()
			go timeout_reqs(restartCh, d)
			break
		case <-restartCh:
			ctx, ctxCancel = context.WithCancel(parentCtx)
			go trd_handler(ctx)
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
