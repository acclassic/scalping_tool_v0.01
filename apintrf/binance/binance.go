package binance

import (
	"encoding/json"
	"fmt"
	"strings"
	"swap-trader/apintrf"
	"sync"

	"golang.org/x/net/websocket"
)

type WsConfig struct {
	WsOrigin  string
	WsAddress string
}

func (wConfig *WsConfig) Connect_ws() *websocket.Conn {
	wsConfig, _ := websocket.NewConfig(wConfig.WsAddress, wConfig.WsOrigin)
	wsConn, err := websocket.DialConfig(wsConfig)
	if err != nil {
		apintrf.Log_err().Panicf("Error connecting to the WebSocket: %s", err)
	}
	return wsConn
}

var api *ApiConfig

type ApiConfig struct {
	ApiKey    string
	SecretKey string
	Address   string
}

func Set_api_config(config *ApiConfig) {
	api = config
}

const (
	BUY  = "BUY"
	SELL = "SELL"
)

func get_order_book(symbol string) BookDepth {
	qParams := queryParams{"symbol": strings.ToUpper(symbol), "limit": "10"}
	resp := get_req("/api/v3/depth", qParams)
	defer resp.Body.Close()
	var orderBook BookDepth
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&orderBook)
	return orderBook
}

type Funds struct {
	Balances []Balance `json:"balances"`
}

type Balance struct {
	Asset  string      `json:"asset"`
	Amount json.Number `json:"free"`
}

func get_funds(asset string) Funds {
	resp := sGet_req("/api/v3/account", nil)
	defer resp.Body.Close()
	var funds Funds
	json.NewDecoder(resp.Body).Decode(&funds)
	fmt.Println(funds)
	return funds
}

func post_order(symbol, side string, q float64) {
	qty := fmt.Sprint(q)
	qParams := queryParams{
		"symbol": symbol,
		"side":   side,
		"type":   "MARKET",
	}
	switch side {
	case BUY:
		qParams["quoteOrderQty"] = qty
	case SELL:
		qParams["quantity"] = qty
	}
	resp := sPost_req("/api/v3/order/test", qParams)
	//TODO check whick struct to implement and what resp needed
	var t interface{}
	json.NewDecoder(resp.Body).Decode(&t)
	fmt.Println(resp)
}

func get_spread_prices(symbol string) []float64 {
	qParams := queryParams{
		"symbol":   strings.ToUpper(symbol),
		"interval": "1d",
		//TODO implement into trdstrategy
		"limit": "6",
	}
	resp := get_req("/api/v3/klines", qParams)
	defer resp.Body.Close()
	var p [][]json.Number
	json.NewDecoder(resp.Body).Decode(&p)
	var prices []float64
	for _, v := range p[:5] {
		p, _ := v[4].Float64()
		prices = append(prices, p)
	}
	return prices
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

// Get order book and cache result. Then listen to the WS for new orders and send result to the reps handler.
func Listen_ws(wsConn *websocket.Conn) {
	//ticker := time.NewTicker(5 * time.Second)
	//go set_avgSpread(ticker)
	var wsResp WsStream
	for {
		err := websocket.JSON.Receive(wsConn, &wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		go resp_hander(&wsResp)
	}
}

type TrdStrategy struct {
	Market     string
	SpreadBase int
	TrdAmount  float64
}

var trdStrategy TrdStrategy

func (strat TrdStrategy) init_order_price(market string, wg *sync.WaitGroup) {
	//defer wg.Done()
	//switch market {
	//case "buy":
	//	oBook := get_order_book(strat.BuyMarket)
	//	price, _ := oBook.Asks[2][0].Float64()
	//	buyMarketP.update_price(price)
	//case "sell":
	//	oBook := get_order_book(strat.SellMarket)
	//	price, _ := oBook.Bids[2][0].Float64()
	//	sellMarketP.update_price(price)
	//}
}

func (strat TrdStrategy) Exec_strat(wsConn *websocket.Conn) {
	//Set TrdStrategy config
	trdStrategy = strat
	//var params []string
	//params = append(params, fmt.Sprintf("%s@depth5", strat.BuyMarket))
	//params = append(params, fmt.Sprintf("%s@depth5", strat.SellMarket))
	//var wg sync.WaitGroup
	//wg.Add(2)
	//go strat.init_order_price("buy", &wg)
	req := WsRequest{
		Method: "SUBSCRIBE",
		Params: []string{fmt.Sprintf("%s@depth10@100ms", trdStrategy.Market), fmt.Sprintf("%s@aggTrade", trdStrategy.Market)},
		Id:     1,
	}
	req.Send_req(wsConn)
	//wg.Wait()
	Listen_ws(wsConn)
}
