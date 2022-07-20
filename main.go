package main

import (
	"swap-trader/apintrf/binance"
)

func main() {
	wsConfig := binance.WsConfig{
		WsOrigin:  "wss://testnet.binance.vision",
		WsAddress: "wss://testnet.binance.vision/stream",
	}
	wsConn := wsConfig.Connect_ws()
	//ApiKey:    "v5eVCtrMZoAaJveVQwOG9615Zq558h9Rt3gHmf2C2gmA0lrmVzLRWUhW3o5JCoq3",
	//SecretKey: "7NYBhdsJaPE2tXxovS2kklYXnfc8dMf6kTYA5W5I1GJ1VuI6zIvg4iTfXN6Tra19",
	apiConfig := binance.ApiConfig{
		ApiKey:    "v5eVCtrMZoAaJveVQwOG9615Zq558h9Rt3gHmf2C2gmA0lrmVzLRWUhW3o5JCoq3",
		SecretKey: "7NYBhdsJaPE2tXxovS2kklYXnfc8dMf6kTYA5W5I1GJ1VuI6zIvg4iTfXN6Tra19",
		Address:   "https://testnet.binance.vision/api",
	}
	//req := binance.WsRequest{
	//	Method: "SUBSCRIBE",
	//	Params: []string{
	//		"btcusdt@depth10",
	//		"btcbusd@depth10",
	//	},
	//	Id: 1,
	//}
	//req.Send_req(wsConn)
	btcUahStrat := binance.TrdMarkets{
		BuyMarket:  "btcusdt",
		SellMarket: "bnbusdt",
	}
	btcUahStrat.Exec_strat(wsConn, &apiConfig)
}
