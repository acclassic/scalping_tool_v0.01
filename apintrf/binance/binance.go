package binance

import (
	"encoding/json"
	"fmt"
	"swap-trader/apintrf"

	"golang.org/x/net/websocket"
)

type ApiConfig struct {
	ApiKey    string
	SecretKey string
	WsOrigin  string
	WsAddress string
}

func (aConfig *ApiConfig) Connect_ws() *websocket.Conn {
	wsConfig, _ := websocket.NewConfig(aConfig.WsAddress, aConfig.WsOrigin)
	wsConn, err := websocket.DialConfig(wsConfig)
	if err != nil {
		apintrf.Log_err().Panicf("Error connecting to the WebSocket: %s", err)
	}
	return wsConn
}

type WsRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	Id     int      `json:"id"`
}

func (req WsRequest) Send_req(wsConn *websocket.Conn) {
	err := websocket.JSON.Send(wsConn, req)

	if err != nil {
		apintrf.Log_err().Printf("Error sending request to WS: %s", err)
	}
}

type WsStream struct {
	Stream string    `json:"stream"`
	Data   BookDepth `json:"data"`
}

type BookDepth struct {
	Bids [][]json.Number `json:"bids"`
	Asks [][]json.Number `json:"asks"`
}

type Order struct {
	Price string
	Qty   string
}

func Listen_ws(wsConn *websocket.Conn, markets TrdStradegy) {
	var wsResp WsStream
	for {
		err := websocket.JSON.Receive(wsConn, &wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		fmt.Println(wsResp)
		//handle_resp(wsResp, markets)
	}
}

type TrdStradegy struct {
	BuyMarket  string
	SellMarket string
}

func (strat TrdStradegy) Exec_strat(wsConn *websocket.Conn) {
	var params []string
	params = append(params, fmt.Sprintf("%s@depth10", strat.BuyMarket))
	params = append(params, fmt.Sprintf("%s@depth10", strat.SellMarket))
	req := WsRequest{
		Method: "SUBSCRIBE",
		Params: params,
		Id:     1,
	}
	req.Send_req(wsConn)
	Listen_ws(wsConn, strat)
}
