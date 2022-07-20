package binance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"swap-trader/apintrf"

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
		//go resp_hander(&wsResp, markets)
	}
}

type TrdMarkets struct {
	BuyMarket  string
	SellMarket string
}

func (markets TrdMarkets) Exec_strat(wsConn *websocket.Conn, apiConf *ApiConfig) {
	var params []string
	params = append(params, fmt.Sprintf("%s@depth10", markets.BuyMarket))
	params = append(params, fmt.Sprintf("%s@depth10", markets.SellMarket))
	buyMBook := apiConf.get_order_book(markets.BuyMarket)
	//TODO implement func to get directly n price of slice
	buyPrice, _ := buyMBook.Asks[2][0].Float64()
	buyMarket.update_price(buyPrice)
	req := WsRequest{
		Method: "SUBSCRIBE",
		Params: params,
		Id:     1,
	}
	req.Send_req(wsConn)
	Listen_ws(wsConn, markets)
}
