package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"swap-trader/apintrf"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

var cancelCh chan time.Duration

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

//TODO change to custom type and use that as func param
const (
	BUY  = "BUY"
	SELL = "SELL"
	CONV = "CONV"
)

var exLimitsCtrs limitsCtrs

type limitsCtrs struct {
	reqWeight rCounter
	rawReq    rCounter
	maxOrders rCounter
	orders    rCounter
}

func rLimits_handler(exLimits limitsCtrs) {
	//Reset counters after Tick
	for {
		select {
		case <-exLimits.reqWeight.ticker:
			exLimits.reqWeight.update_counter(exLimitsCtrs.reqWeight.resetCount)
		case <-exLimits.orders.ticker:
			exLimits.orders.update_counter(exLimitsCtrs.orders.resetCount)
		case <-exLimits.maxOrders.ticker:
			exLimits.maxOrders.update_counter(exLimitsCtrs.maxOrders.resetCount)
		case <-exLimits.rawReq.ticker:
			exLimits.rawReq.update_counter(exLimitsCtrs.rawReq.resetCount)
		}
	}
}

type rCounter struct {
	count      int
	resetCount int
	ticker     <-chan time.Time
	mu         sync.RWMutex
}

func (c *rCounter) init_counter(n int, interval time.Duration) {
	c.count = n
	c.resetCount = n
	c.ticker = time.Tick(interval)
}

func (c *rCounter) update_counter(n int) {
	c.mu.Lock()
	c.count = c.count - n
	c.mu.Unlock()
}

func (c *rCounter) get_counter() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.count
}

func parse_limit_duration(n int, unit string) time.Duration {
	var u string
	switch unit {
	case "SECOND":
		u = "s"
	case "MINUTE":
		u = "m"
	case "DAY":
		d := time.Duration(n*24) * time.Hour
		return d
	}
	duration := fmt.Sprintf("%d%s", n, u)
	d, _ := time.ParseDuration(duration)
	return d
}

func set_rLimits(rLimits []RLimits) {
	//Init counters and create tickers
	for _, limit := range rLimits {
		switch limit.RType {
		case "REQUEST_WEIGHT":
			interval := parse_limit_duration(limit.IntervalNum, limit.Interval)
			exLimitsCtrs.reqWeight.init_counter(limit.Limit, interval)
		case "ORDERS":
			if limit.Interval != "DAY" {
				interval := parse_limit_duration(limit.IntervalNum, limit.Interval)
				exLimitsCtrs.orders.init_counter(limit.Limit, interval)
			} else if limit.Interval == "DAY" {
				interval := parse_limit_duration(limit.IntervalNum, limit.Interval)
				exLimitsCtrs.maxOrders.init_counter(limit.Limit, interval)
			}
		case "RAW_REQUESTS":
			interval := parse_limit_duration(limit.IntervalNum, limit.Interval)
			exLimitsCtrs.rawReq.init_counter(limit.Limit, interval)
		}
	}
}

var symbolsFilters []MarketEx

type ExInfo struct {
	RateLimits []RLimits  `json:"rateLimits"`
	Symbols    []MarketEx `json:"symbols"`
}

type RLimits struct {
	RType       string `json:"rateLimitType"`
	Interval    string `json:"interval"`
	IntervalNum int    `json:"intervalNum"`
	Limit       int    `json:"limit"`
}

type MarketEx struct {
	Symbol  string      `json:"symbol"`
	Filters []ExFilters `json:"filters"`
}

type ExFilters struct {
	FType         string  `json:"filterType"`
	MinPrice      float64 `json:"minPrice,string"`
	MaxPrice      float64 `json:"maxPrice,string"`
	TickSize      float64 `json:"tickSize,string"`
	MultipUp      float64 `json:"multipliererUp,string"`
	MultipDown    float64 `json:"multipliererDown,string"`
	AvgPriceMins  int     `json:"avgPriceMins,string"`
	MinQty        float64 `json:"minQty,string"`
	MaxQty        float64 `json:"maxQty,string"`
	StepSize      float64 `json:"stepSize,string"`
	MinNotional   float64 `json:"minNotional,sting"`
	ApplyToMarket bool    `json:"applyToMarket,string"`
	MaxNumOrders  int     `json:"maxNumOrders,string"`
}

func get_ex_info(buyMarket, sellMarket, convMarket string) ExInfo {
	symbols := fmt.Sprintf(`["%s","%s","%s"]`, buyMarket, sellMarket, convMarket)
	qParams := queryParams{
		"symbols": symbols,
	}
	resp := get_req("/api/v3/exchangeInfo", qParams)
	defer resp.Body.Close()
	var exInfo ExInfo
	json.NewDecoder(resp.Body).Decode(&exInfo)
	return exInfo
}

type BookDepth struct {
	Bids [][]json.Number `json:"bids"`
	Asks [][]json.Number `json:"asks"`
}

func get_order_book(symbol string) BookDepth {
	qParams := queryParams{"symbol": strings.ToUpper(symbol), "limit": "3"}
	resp := get_req("/api/v3/depth", qParams)
	defer resp.Body.Close()
	var orderBook BookDepth
	//TODO maybe change json decoder to one liner
	dec := json.NewDecoder(resp.Body)
	dec.Decode(&orderBook)
	return orderBook
}

type Funds struct {
	Balances []Balance `json:"balances"`
}

type Balance struct {
	Asset  string  `json:"asset"`
	Amount float64 `json:"free,string"`
}

//TODO check this function live
func get_funds(asset string) (float64, error) {
	resp := sGet_req("/api/v3/account", nil)
	defer resp.Body.Close()
	var funds Funds
	json.NewDecoder(resp.Body).Decode(&funds)
	for _, v := range funds.Balances {
		if v.Asset == asset {
			return v.Amount, nil
		}
	}
	err := errors.New("Asset not fund!")
	return 0, err
}

type order struct {
	buyAmnt  float64
	sellAmnt float64
}

func format_price(price float64) {
	//TODO implement func to format price with filters
}

func market_order(symbol, side string, qty float64) {
	qParams := queryParams{
		"symbol": symbol,
		"side":   side,
		"type":   "MARKET",
	}
	switch side {
	case BUY:
		qParams["quoteOrderQty"] = fmt.Sprint(qty)
	case SELL:
		qParams["quantity"] = fmt.Sprint(qty)
	}
	resp := sPost_req("/api/v3/order/test", qParams)
	//TODO check whick struct to implement and what resp needed
	var t interface{}
	json.NewDecoder(resp.Body).Decode(&t)
	fmt.Println(resp)
}

func limit_order(symbol, side string, qty, price float64) {
	//TODO check if time in force needs to be var
	qParams := queryParams{
		"symbol":      symbol,
		"side":        side,
		"type":        "LIMIT",
		"timeInForce": "IOC",
		"price":       fmt.Sprint(price),
		"quantity":    fmt.Sprint(qty),
	}
	resp := sPost_req("/api/v3/order/test", qParams)
	//TODO check whick struct to implement and what resp needed
	var t interface{}
	json.NewDecoder(resp.Body).Decode(&t)
	fmt.Println(resp)
}

type WsRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	Id     int      `json:"id"`
}

type WsStream struct {
	Stream string    `json:"stream"`
	Data   BookDepth `json:"data"`
}

func (req WsRequest) Send_req(wsConn *websocket.Conn) {
	err := websocket.JSON.Send(wsConn, req)

	if err != nil {
		apintrf.Log_err().Printf("Error sending request to WS: %s", err)
	}
}

func subscribeStream(wsConn *websocket.Conn, markets ...string) {
	var reqParams []string
	for _, market := range markets {
		param := fmt.Sprintf("%s@depth5@100ms", strings.ToLower(market))
		reqParams = append(reqParams, param)
	}
	wsReq := WsRequest{
		Method: "SUBSCRIBE",
		Params: reqParams,
		Id:     1,
	}
	wsReq.Send_req(wsConn)
}

type TT struct {
	Data T `json:"data"`
}
type T struct {
	S  int64  `json:"E"`
	SS string `json:"e"`
}

// Get order book and cache result. Then listen to the WS for new orders and send result to the reps handler.
func listen_ws(ctx context.Context, stopCh <-chan bool, wsConn *websocket.Conn) {
	//var wsResp TT
	var wsResp WsStream
	//TODO add ctx value origin. Not needed but good
	ctx, cancel := context.WithCancel(ctx)
	for {
		err := websocket.JSON.Receive(wsConn, &wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		resp_hander(ctx, &wsResp)

		//select {
		//case <-stopCh:
		//	cancel()
		//	return
		//default:
		//	ctx = context.WithValue(ctx, "id", i)
		//	err := websocket.JSON.Receive(wsConn, &wsResp)
		//	//TODO how should this err be handled? Fatal? Retry after timeout?
		//	if err != nil {
		//		apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		//	}
		//	resp_hander(ctx, &wsResp)
		//	fmt.Printf("Ctx %d send\n", i)
		//}
	}
}

func init_order_price(market string, symbol string, pos int) {
	oBook := get_order_book(symbol)
	switch market {
	case BUY:
		price, _ := oBook.Asks[pos][0].Float64()
		buyMarketP.update_price(price)
	case SELL:
		price, _ := oBook.Bids[pos][0].Float64()
		sellMarketP.update_price(price)
	case CONV:
		price, _ := oBook.Asks[pos][0].Float64()
		convMarketP.update_price(price)
	}
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

var trdStrategy TrdStratConfig

type TrdStratConfig struct {
	BuyMarket  string
	SellMarket string
	ConvMarket string
	TrdRate    float64
}

func (strat TrdStratConfig) Exec_strat(wsConn *websocket.Conn) {
	//Init cancelCh
	cancelCh = make(chan time.Duration)
	ctx := context.Background()
	//TODO implement exInfo ticker 24h and other counters
	//Set TrdStrategy config
	trdStrategy = strat
	//Set ExInfos
	//exInfos := get_ex_info(strat.BuyMarket, strat.SellMarket, strat.ConvMarket)
	//symbolsFilters = exInfos.Symbols
	//set_rLimits(exInfos.RateLimits)
	//go rLimits_handler(exLimitsCtrs)
	//TODO implement goroutine
	//Init Maret prices
	//init_order_price(BUY, strat.BuyMarket, 1)
	//init_order_price(SELL, strat.SellMarket, 1)
	//init_order_price(CONV, strat.ConvMarket, 2)
	//Subscribe to WS book stream
	//subscribeStream(wsConn, strat.BuyMarket, strat.SellMarket, strat.ConvMarket)
	listen_ws(ctx, wsConn)
	//TODO see if needed 429 HANDLER
	//wsReq := WsRequest{
	//	Method: "SUBSCRIBE",
	//	Params: []string{"btcusdt@aggTrade"},
	//	Id:     1,
	//}
	//wsReq.Send_req(wsConn)
	//go func() {
	//	time.Sleep(2 * time.Second)
	//	fmt.Println("sleep func run")
	//	cancelCh <- time.Duration(10) * time.Second
	//}()
	//ch := make(chan bool)
	//go listen_ws(ctx, ch, wsConn)
	//for {
	//	select {
	//	case interval := <-cancelCh:
	//		fmt.Println(interval)
	//		ch <- false
	//		go timeout_conns(ch, interval)
	//	case <-ch:
	//		go listen_ws(ctx, ch, wsConn)
	//	}
	//}
}

//TODO see if needed
//func timeout_conns(ch chan<- bool, d time.Duration) {
//	ticker := time.NewTicker(d)
//	defer ticker.Stop()
//	for {
//		select {
//		case <-ticker.C:
//			ch <- true
//			return
//		}
//	}
//}
