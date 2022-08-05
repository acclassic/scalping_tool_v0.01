package main

import (
	"swap-trader/apintrf/binance"
)

func main() {
	//wsConfig := binance.WsConfig{
	//	WsOrigin:  "wss://testnet.binance.vision",
	//	WsAddress: "wss://testnet.binance.vision/stream",
	//}
	wsConfig := binance.WsConfig{
		WsOrigin:  "wss://stream.binance.com:9443",
		WsAddress: "wss://stream.binance.com:9443/stream",
	}
	wsConn := wsConfig.Connect_ws()
	apiConfig := binance.ApiConfig{
		ApiKey:    "RJOKv5gORTbamrrlbuy18M5tSQ54plDxw30oZksoikOphSuyTUwyboewMTqZa7UE",
		SecretKey: "wXU9BitdNlPoYOkGACRNN6ZaVOJzfTR0uKCgwPNpKkeV0rqhaPo7Lzdh13LQRksu",
		Address:   "https://testnet.binance.vision",
	}
	binance.Set_api_config(&apiConfig)
	btcUahStrat := binance.TrdStrategy{
		Market:     "btcusdt",
		SpreadBase: 5,
		TrdAmount:  10.0,
	}
	btcUahStrat.Exec_strat(wsConn)
}
