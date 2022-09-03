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
		WsOrigin:  "wss://stream.binance.com",
		WsAddress: "wss://stream.binance.com:9443/stream",
	}
	wsConn := wsConfig.Connect_ws()
	//apiConfig := binance.ApiConfig{
	//	ApiKey:    "RJOKv5gORTbamrrlbuy18M5tSQ54plDxw30oZksoikOphSuyTUwyboewMTqZa7UE",
	//	SecretKey: "wXU9BitdNlPoYOkGACRNN6ZaVOJzfTR0uKCgwPNpKkeV0rqhaPo7Lzdh13LQRksu",
	//	Address:   "https://testnet.binance.vision",
	//}
	apiConfig := binance.ApiConfig{
		ApiKey:    "RJOKv5gORTbamrrlbuy18M5tSQ54plDxw30oZksoikOphSuyTUwyboewMTqZa7UE",
		SecretKey: "wXU9BitdNlPoYOkGACRNN6ZaVOJzfTR0uKCgwPNpKkeV0rqhaPo7Lzdh13LQRksu",
		Address:   "https://api.binance.com",
	}
	binance.Set_api_config(&apiConfig)
	trdStrat := binance.TrdStratConfig{
		BuyMarket:  "BNBEUR",
		SellMarket: "BNBUSDT",
		ConvMarket: "EURUSDT",
		TrdRate:    0.1,
	}
	trdStrat.Exec_strat(wsConn)
	binance.App_handler(trdStrat, wsConfig)
}
