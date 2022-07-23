package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type ApiConfig struct {
	ApiKey    string
	SecretKey string
	Address   string
}

func (a *ApiConfig) get_order_book(symbol string) BookDepth {
	u := fmt.Sprintf("%s/v3/depth", a.Address)
	rUrl, _ := url.Parse(u)
	qParams := url.Values{}
	qParams.Add("symbol", strings.ToUpper(symbol))
	qParams.Add("limit", "10")
	rUrl.RawQuery = qParams.Encode()
	resp, err := http.Get(rUrl.String())
	defer resp.Body.Close()
	if err != nil {
		//TODO see if err needs to cache and if so what to do.
		fmt.Println(err)
	}
	var orderBook BookDepth
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&orderBook)
	return orderBook
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
func Listen_ws(wsConn *websocket.Conn, markets TrdMarkets) {
	var wsResp WsStream
	for {
		err := websocket.JSON.Receive(wsConn, &wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		go resp_hander(&wsResp, markets)
	}
}

type TrdMarkets struct {
	BuyMarket  string
	SellMarket string
}

func (markets TrdMarkets) init_order_price(market string, api *ApiConfig, wg *sync.WaitGroup) {
	defer wg.Done()
	switch market {
	case "buy":
		oBook := api.get_order_book(markets.BuyMarket)
		price, _ := oBook.Asks[2][0].Float64()
		buyMarketP.update_price(price)
	case "sell":
		oBook := api.get_order_book(markets.SellMarket)
		price, _ := oBook.Bids[2][0].Float64()
		sellMarketP.update_price(price)
	}
}

func (markets TrdMarkets) Exec_strat(wsConn *websocket.Conn, apiConf *ApiConfig) {
	var params []string
	params = append(params, fmt.Sprintf("%s@depth5", markets.BuyMarket))
	params = append(params, fmt.Sprintf("%s@depth5", markets.SellMarket))
	var wg sync.WaitGroup
	wg.Add(2)
	go markets.init_order_price("buy", apiConf, &wg)
	go markets.init_order_price("sell", apiConf, &wg)
	req := WsRequest{
		Method: "SUBSCRIBE",
		Params: params,
		Id:     1,
	}
	req.Send_req(wsConn)
	wg.Wait()
	Listen_ws(wsConn, markets)
}
