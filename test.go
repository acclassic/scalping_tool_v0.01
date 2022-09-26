package main

import (
	"fmt"
	"time"

	"golang.org/x/net/websocket"
)

type WsRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	Id     int      `json:"id"`
}

func main() {
	wsConf, _ := websocket.NewConfig("wss://testnet.binance.vision/ws", "wss://testnet.binance.vision")
	wsConn, _ := websocket.DialConfig(wsConf)
	wsReq := WsRequest{
		Method: "SUBSCRIBE",
		Params: []string{"btcusdt@kline_1s"},
		Id:     1,
	}
	websocket.JSON.Send(wsConn, wsReq)

	var resp interface{}
	for {
		go func() {
			time.Sleep(8 * time.Second)
			err := wsConn.Close()
			fmt.Println(err)
		}()
		websocket.JSON.Receive(wsConn, &resp)
		fmt.Println(resp)
		time.Sleep(1 * time.Second)
	}
}
