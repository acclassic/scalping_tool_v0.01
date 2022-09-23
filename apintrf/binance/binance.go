package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"swap-trader/apintrf"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

//TODO see if trdMarket needs to be new type
const (
	BUY  trdMarket = "BUY"
	SELL trdMarket = "SELL"
	CONV trdMarket = "CONV"
)

var cancelCh = make(chan time.Duration)
var trdFunds accFunds

//TODO use this to check if the weight and req has to be counted against the ctrs
type ctxKey string

type trdMarket string

type accFunds struct {
	amount float64
	mu     sync.RWMutex
}

func (f *accFunds) get_funds(part float64) float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.amount * part
}

func (f *accFunds) update_funds(origin trdMarket, amount float64) {
	f.mu.Lock()
	switch origin {
	case BUY:
		f.amount = f.amount - amount
	case CONV:
		f.amount = f.amount + amount
	}
	f.mu.Unlock()
}

type WsConfig struct {
	WsOrigin  string
	WsAddress string
}

func connect_ws() *websocket.Conn {
	config := get_ws_config()
	wsConfig, _ := websocket.NewConfig(config.WsAddress, config.WsOrigin)
	wsConn, err := websocket.DialConfig(wsConfig)
	if err != nil {
		apintrf.Sys_Logger().Fatalf("WARNING: Could not connect to WS. %s", err)
	}
	return wsConn
}

func get_ws_config() WsConfig {
	file, err := os.Open("config/ws.conf")
	if err != nil {
		apintrf.Sys_Logger().Fatalf("WARNING: Could not load WS config file. %s", err)
	}
	var wsConfig WsConfig
	err := json.NewDecoder(file).Decode(&wsConfig)
	if err != nil {
		apintrf.Sys_Logger().Fatalf("WARNING: Could not decode WS config file. %s", err)
	}
	return wsConfig
}

var api *ApiConfig

type ApiConfig struct {
	ApiKey    string
	SecretKey string
	Address   string
}

func set_api_config() {
	file, err := os.Open("config/api.conf")
	if err != nil {
		apintrf.sys_Logger().Fatalf("WARNING: Could not load API config file. %s", err)
	}
	err := json.NewDecoder(file).Decode(api)
	if err != nil {
		apintrf.sys_Logger().Fatalf("WARNING: Could not decode API config file. %s", err)
	}
}

var exLimitsCtrs limitsCtrs

//TODO check how this is implemented.
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

type limitsCtrs struct {
	reqWeight rCounter
	rawReq    rCounter
	maxOrders rCounter
	orders    rCounter
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

func (c *rCounter) decrease_counter(n int) {
	c.mu.lock()
	c.count = c.count - n
	c.mu.unlock()
}

func (c *rCounter) increase_counter(n int) {
	c.mu.lock()
	c.count = c.count + n
	c.mu.unlock()
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

var symbolsFilters = make(map[string]map[string]ExFilters)

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

//TODO test if TickSize is populated
//TODO implement applyMinToMarket and applyMaxToMarket?
type ExFilters struct {
	FType         string  `json:"filterType"`
	MinPrice      float64 `json:"minPrice,string"`
	MaxPrice      float64 `json:"maxPrice,string"`
	TickSize      string  `json:"tickSize"`
	MultipUp      float64 `json:"multipliererUp,string"`
	MultipDown    float64 `json:"multipliererDown,string"`
	AvgPriceMins  int     `json:"avgPriceMins"`
	MinQty        float64 `json:"minQty,string"`
	MaxQty        float64 `json:"maxQty,string"`
	StepSize      string  `json:"stepSize"`
	MaxNotional   float64 `json:"maxNotional,string"`
	MinNotional   float64 `json:"minNotional,sting"`
	ApplyToMarket bool    `json:"applyToMarket"`
	ApplyMinToM   bool    `json:"applyMinToMarket"`
	ApplyMaxToM   bool    `json:"applyMaxToMarket"`
	MaxNumOrders  int     `json:"maxNumOrders,string"`
	pricePrc      int
	lotPrc        int
	mLotPrc       int
}

//TODO change return to pointer
func get_ex_info(ctx context.Context, buyMarket, sellMarket, convMarket string) (ExInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	symbols := fmt.Sprintf(`["%s","%s","%s"]`, buyMarket, sellMarket, convMarket)
	qParams := queryParams{
		"symbols": symbols,
	}
	req := create_httpReq(http.MethodGet, "/api/v3/exchangeInfo", qParams, false, weightExInfo)
	resp := http_req_handler(ctx, req)
	defer resp.Body.Close()
	var exInfo ExInfo
	json.NewDecoder(resp.Body).Decode(&exInfo)
	return exInfo, nil
}

func set_symbols_filters(marketsFilter []MarketEx) {
	//Convert struct to map with struct for direct access to filters
	for _, v := range marketsFilter {
		for _, f := range v.Filters {
			pricePrc := calc_precision(f.TickSize)
			switch f.FType {
			case "LOT_SIZE":
				lotPrc := calc_precision(f.StepSize)
			case "MARKET_LOT_SIZE":
				mLotPrc := calc_precision(f.StepSize)
			}
			f.pricePrc = pricePrc
			f.lotPrc = lotPrc
			symbolsFilters[v.Symbol][f.FType] = f
		}
	}
}

func calc_precision(tickSize string) int {
	i := 0
	s := strings.SplitAfter(tickSize, ".")
	for _, v := range s[1] {
		i++
		if v == "1" {
			return i
		}
	}
	return 0
}

type BookDepth struct {
	Bids [][]json.Number `json:"bids"`
	Asks [][]json.Number `json:"asks"`
}

func get_order_book(ctx context.Context, symbol string) (BookDepth, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	qParams := queryParams{"symbol": strings.ToUpper(symbol), "limit": "3"}
	req := create_httpReq(http.MethodGet, "/api/v3/depth", qParams, false, weightOrdBook)
	resp := http_req_handler(ctx, req)
	defer resp.Body.Close()
	var orderBook BookDepth
	//TODO maybe change json decoder to one liner
	json.NewDecoder(resp.Body).Decode(&orderBook)
	return orderBook, nil
}

type Funds struct {
	Balances []Balance `json:"balances"`
}

type Balance struct {
	Asset  string  `json:"asset"`
	Amount float64 `json:"free,string"`
}

//TODO test this on live to see if Fiat is aviable
func get_acc_funds(ctx context.Context, asset string) (float64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	req := create_httpReq(http.MethodGet, "/api/v3/account", queryParams{}, false, weightAccInfo)
	resp := http_req_handler(ctx, req)
	defer resp.Body.Close()
	var funds Funds
	json.NewDecoder(resp.Body).Decode(&funds)
	for _, v := range funds.Balances {
		if v.Asset == asset {
			return v.Amount, nil
		}
	}
	err := fmt.Errorf("Could not find %s in funds.", asset)
	return 0, err
}

func get_avg_price(ctx context.Context, symbol string) (float64, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	qParams := queryParams{"symbol": symbol}
	req := create_httpReq(http.MethodGet, "/api/v3/avgPrice", qParams, false, weightAvgPrice)
	resp := http_req_handler(ctx, req)
	defer req.body.Close()
	var avgPrice map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&avgPrice)
	price := avgPrice["price"].(float64)
	return price, nil
}

type OrderResp struct {
	Symbol  string  `json:"symbol"`
	Price   float64 `json:"price,string"`
	Qty     float64 `json:"executedQty,string"`
	Status  string  `json:"status"`
	stratID int     `json:"strategyId"`
}

func market_order(ctx context.Context, symbol string, side trdMarket, qty float64) (OrderResp, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	//TODO test this
	qParams := queryParams{
		"symbol":           symbol,
		"side":             string(side),
		"type":             "MARKET",
		"newOrderRespType": "RESULT",
	}
	switch side {
	case BUY:
		qParams["quoteOrderQty"] = fmt.Sprint(qty)
	case SELL:
		qParams["quantity"] = fmt.Sprint(qty)
	}
	req := create_httpReq(http.MethodPost, "/api/v3/order/test", qParams, true, weightOrder)
	resp := http_req_handler(ctx, req)
	//TODO check whick struct to implement and what resp needed
	var order OrderResp
	json.NewDecoder(resp.Body).Decode(&order)
	return order, nil
}

//TODO change side from string to trdMarket. Probably need to implement type
//TODO implement strategyId for analytics. Pass form trd_handler()
func limit_order(ctx context.Context, symbol string, side trdMarket, price, qty float64) (OrderResp, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	//TODO check if time in force needs to be var
	qParams := queryParams{
		"symbol":           symbol,
		"side":             string(side),
		"type":             "LIMIT",
		"timeInForce":      "IOC",
		"price":            fmt.Sprint(price),
		"quantity":         fmt.Sprint(qty),
		"newOrderRespType": "RESULT",
	}
	req := create_httpReq(http.MethodPost, "/api/v3/order/test", qParams, true, weightOrder)
	resp := http_req_handler(ctx, req)
	//TODO check whick struct to implement and what resp needed
	var order OrderResp
	json.NewDecoder(resp.Body).Decode(&order)
	return order, nil
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
func listen_ws(wsConn *websocket.Conn) {
	var wsResp WsStream
	for {
		err := websocket.JSON.Receive(wsConn, &wsResp)
		if err != nil {
			apintrf.Log_err().Panicf("Error reading request from WS: %s", err)
		}
		resp_hander(&wsResp)
	}
}

func init_markets_price(ctx context.Context, symbol string, wg *sync.WaitGroup) {
	defer wg.Done()
	oBook := get_order_book(ctx, symbol)
	switch symbol {
	case trdStrategy.BuyMarket:
		price, _ := oBook.Asks[pos][0].Float64()
		buyMarketP.update_price(price)
	case trdStrategy.SellMarket:
		price, _ := oBook.Bids[pos][0].Float64()
		sellMarketP.update_price(price)
	case trdStrategy.ConvMarket:
		price, _ := oBook.Asks[pos][0].Float64()
		convMarketP.update_price(price)
	}
}

var trdStrategy TrdStratConfig

type TrdStratConfig struct {
	BuyMarket  string
	SellMarket string
	ConvMarket string
	TrdRate    float64
}

func set_trd_strat() {
	file, err := os.Open("config/strat.conf")
	if err != nil {
		apintrf.sys_Logger().Fatalf("WARNING: Could not load API config file. %s", err)
	}
	err := json.NewDecoder(file).Decode(&trdStrategy)
	if err != nil {
		apintrf.sys_Logger().Fatalf("WARNING: Could not decode API config file. %s", err)
	}
}

//TODO overthink if ctx value is needed
//TODO implement reconect after 24h to websocket.
func Exec_strat() {
	//Set Api Config
	set_api_config()
	//Set Trd Strat
	set_trd_strat()
	//Get WS Connection
	ws := connect_ws()
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKey("reqWeight"), true)
	//Set ExInfos
	exInfos := get_ex_info(ctx, strat.BuyMarket, strat.SellMarket, strat.ConvMarket)
	set_symbols_filters(exInfos.Symbols)
	set_rLimits(exInfos.RateLimits)
	//Set accFunds
	funds := get_acc_funds(ctx, "EUR")
	trdFunds = funds
	//Init Maret prices
	var wg sync.WaitGroup
	wg.Add(3)
	go init_markets_price(ctx, trdStrategy.BuyMarket, wg)
	go init_markets_price(ctx, trdStrategy.SellMarket, wg)
	go init_markets_price(ctx, trdStrategy.ConvMarket, wg)
	wg.Wait()
	//Subscribe to WS book stream
	subscribeStream(wsConn, strat.BuyMarket, strat.SellMarket, strat.ConvMarket)
	//Start service handler
	service_handler(ctx, ws)
}
