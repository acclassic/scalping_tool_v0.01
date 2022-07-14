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
	jsonReq, _ := json.Marshal(req)
	_, err := wsConn.Write(jsonReq)
	if err != nil {
		apintrf.Log_err().Printf("Error sending request to WS: %s", err)
	}
}

func Listen_ws(wsConn *websocket.Conn, markets TrdStradegy) {
	var wsResp = make([]byte, 1024)
	for {
		_, err := wsConn.Read(wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		handle_resp(wsResp, markets)
		//fmt.Printf("%s\n", wsResp)
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
