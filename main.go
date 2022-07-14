package main

import (
	"swap-trader/apintrf/binance"
)

type WsRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	Id     int      `json:"id"`
}

func connect_wss() {
	//wsResp := make([]byte, 1024)
	//for {
	//	_, e := wsConn.Read(wsResp)
	//	fmt.Println(e)
	//	fmt.Printf("%s\n", wsResp)
	//}
}

func main() {
	apiConfig := binance.ApiConfig{
		ApiKey:    "v5eVCtrMZoAaJveVQwOG9615Zq558h9Rt3gHmf2C2gmA0lrmVzLRWUhW3o5JCoq3",
		SecretKey: "7NYBhdsJaPE2tXxovS2kklYXnfc8dMf6kTYA5W5I1GJ1VuI6zIvg4iTfXN6Tra19",
		WsOrigin:  "wss://testnet.binance.vision",
		WsAddress: "wss://testnet.binance.vision/stream",
	}
	wsConn := apiConfig.Connect_ws()
	//req := binance.WsRequest{
	//	Method: "SUBSCRIBE",
	//	Params: []string{
	//		"btcusdt@depth10",
	//		"btcbusd@depth10",
	//	},
	//	Id: 1,
	//}
	//req.Send_req(wsConn)
	btcUahStrat := binance.TrdStradegy{
		BuyMarket:  "btcusdt",
		SellMarket: "btcbusd",
	}
	btcUahStrat.Exec_strat(wsConn)
}
