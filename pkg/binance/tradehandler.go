package binance

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"swap-trader/pkg/log"
	"sync"
)

var stratId idCtr

type idCtr struct {
	id int
	mu sync.Mutex
}

func (id *idCtr) init_stratId() {
	id.mu.Lock()
	id.id = 0
	id.mu.Unlock()
}

func (id *idCtr) get_stratId() int {
	id.mu.Lock()
	defer id.mu.Unlock()
	id.id = id.id + 1
	return id.id
}

type trdInfo struct {
	buyPrice     float64
	sellAmnt     float64
	sellAvgPrice float64
	convAvgPrice float64
	convAmnt     float64
	stratId      int
	reqWeight    int
	rawReqs      int
	orders       int
}

func trd_handler(ctx context.Context) {
	//TODO check if possible to only get avgPrice once and pass to other funcs.
	//TODO check if chan needs init func for variable buffer size
	trdCh := make(chan bool, 3)
	for {
		trdSignal, buyPrice := trd_signal()
		if trdSignal == true {
			select {
			case trdCh <- true:
				//Check if ctx status before starting trd
				if ctx.Err() != nil {
					return
				}
				trd := trdInfo{
					reqWeight: weightTrd,
					rawReqs:   5,
					orders:    3,
					buyPrice:  buyPrice,
					stratId:   stratId.get_stratId(),
				}
				//Block limitsCtrs to ensure trd execution. Trd ctrs will be used to free exLimitsCtrs in case of an error.
				exLimitsCtrs.reqWeight.decrease_counter(trd.reqWeight)
				exLimitsCtrs.rawReq.decrease_counter(trd.rawReqs)
				exLimitsCtrs.orders.decrease_counter(trd.orders)
				exLimitsCtrs.maxOrders.decrease_counter(trd.orders)

				ctx = context.WithValue(ctx, ctxKey("reqWeight"), false)
				err := buy_order(ctx, &trd)
				if err != nil {
					unblock_limit_ctrs(&trd)
					log.Strat_logger().Println(err)
					<-trdCh
				}

				// If order can't be sold retry 3 times. If sold continue to conv_order or drop trd and free chan.
				err = sell_order(ctx, &trd)
				if err != nil {
					log.Strat_logger().Fatalln(err)
					//log.Strat_logger().Print(err)
					err := retry_order(ctx, SELL, &trd)
					if err != nil {
						unblock_limit_ctrs(&trd)
						log.Strat_logger().Println(err)
						<-trdCh
					}
				}

				// If order can't be sold retry 3 times. If sold continue to log and end trd or drop trd and free chan.
				err = conv_order(ctx, &trd)
				if err != nil {
					log.Strat_logger().Fatalln(err)
					//log.Strat_logger().Print(err)
					err := retry_order(ctx, CONV, &trd)
					if err != nil {
						log.Strat_logger().Println(err)
						<-trdCh
					}
				}

				<-trdCh
				break
			case <-ctx.Done():
				//If ctx is cancelled exit func
				return
			default:
				//If trdCh is full drop chan
				break
			}
		}
	}
}

func buy_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	price := 25.0
	//ordParams.price = trdFunds.get_funds(trdStrategy.TrdRate)
	qty := price / trd.buyPrice
	//qty := price / trd.buyPrice * 0.75

	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
	qty, err := parse_market_qty(ctx, qty, trdStrategy.BuyMarket)
	if err != nil {
		return err
	}
	err = parse_market_min_notional(ctx, price, qty, trdStrategy.BuyMarket)
	if err != nil {
		return err
	}
	order, err := limit_order(ctx, trdStrategy.BuyMarket, price, trd.stratId)
	update_trd_order(weightOrder, trd)
	if err != nil {
		return err
	}
	if order.Status == "REJECTED" {
		err := errors.New("Order was rejected.")
		return err
	}

	if len(order.Fills) < 1 {
		err := errors.New("Order was not filled")
		return err
	}
	//Calc sellAmnt
	var sellAmnt float64
	for _, fill := range order.Fills {
		sellAmnt = (fill.Qty - fill.Comm) + sellAmnt
	}

	//Calc the price
	var orderPrice float64
	for _, fill := range order.Fills {
		orderPrice = orderPrice + (fill.Price * fill.Qty)
	}

	//Write result to analytics file.
	log.Add_analytics(order.StratID, order.Symbol, orderPrice, sellAmnt)
	//Update trd
	trd.buyPrice = order.Price
	trd.sellAmnt = sellAmnt
	return nil
}

//TODO check this on testnet
//func buy_order(ctx context.Context, trd *trdInfo) error {
//	if ctx.Err() != nil {
//		return ctx.Err()
//	}
//	ctx, cancel := context.WithCancel(ctx)
//	var ordParams orderParams
//	ordParams.price = 70
//	//ordParams.price = trdFunds.get_funds(trdStrategy.TrdRate)
//	ordParams.qty = ordParams.price / trd.buyPrice * 0.75
//
//	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
//	priceCh := make(chan float64)
//	qtyCh := make(chan float64)
//	errCh := make(chan error)
//	go parse_price(ctx, errCh, priceCh, ordParams, trdStrategy.BuyMarket)
//	go parse_qty(ctx, errCh, qtyCh, ordParams, trdStrategy.BuyMarket)
//	select {
//	case price := <-priceCh:
//		select {
//		case qty := <-qtyCh:
//			err := parse_min_notional(ctx, ordParams, trdStrategy.BuyMarket)
//			if err != nil {
//				return err
//			}
//			order, err := limit_order(ctx, trdStrategy.BuyMarket, BUY, price, qty, trd.stratId)
//			update_trd_order(weightOrder, trd)
//			if err != nil {
//				return err
//			}
//			if order.Status == "REJECTED" {
//				err := errors.New("Order was rejected.")
//				return err
//			}
//
//			if len(order.Fills) < 1 {
//				err := errors.New("Order was not filled")
//				return err
//			}
//			//Write result to analytics file. No need to wait before return
//			log.Add_analytics(order.StratID, order.Symbol, order.Price, order.Qty)
//			trd.buyPrice = ordParams.price
//			trd.sellAmnt = order.Qty
//			return nil
//		case err := <-errCh:
//			cancel()
//			return err
//		}
//	case err := <-errCh:
//		cancel()
//		return err
//	}
//}

func sell_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	qty, err := parse_market_qty(ctx, trd.sellAmnt, trdStrategy.SellMarket)
	if err != nil {
		return err
	}
	if trd.sellAvgPrice == 0 {
		price, err := get_avg_price(ctx, trdStrategy.SellMarket)
		update_trd_req(weightAvgPrice, trd)
		if err != nil {
			return err
		}
		trd.sellAvgPrice = price
	}
	err = parse_market_min_notional(ctx, trd.sellAvgPrice, qty, trdStrategy.SellMarket)
	if err != nil {
		return err
	}
	order, err := market_order(ctx, trdStrategy.SellMarket, SELL, qty, trd.stratId)
	update_trd_order(weightOrder, trd)
	if err != nil {
		return err
	}
	if order.Status == "REJECTED" {
		err := errors.New("Order was rejected.")
		return err
	}
	//Calc the convAmnt
	var convAmnt float64
	for _, fill := range order.Fills {
		convAmnt = convAmnt + (fill.Price * fill.Qty)
	}
	//Write result to analytics file.
	log.Add_analytics(order.StratID, order.Symbol, convAmnt, order.Qty)
	//Update trd
	trd.convAmnt = convAmnt
	return nil
}

func conv_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	qty := trd.convAmnt

	qty, err := parse_market_qty(ctx, trd.convAmnt, trdStrategy.ConvMarket)
	if err != nil {
		return err
	}
	if trd.convAvgPrice == 0 {
		//No need to check price or lot filter because by using quoteOrderQty the lot filter is respected.
		price, err := get_avg_price(ctx, trdStrategy.ConvMarket)
		update_trd_req(weightAvgPrice, trd)
		if err != nil {
			return err
		}
		trd.convAvgPrice = price
	}
	err = parse_market_min_notional(ctx, trd.convAvgPrice, qty, trdStrategy.ConvMarket)
	if err != nil {
		return err
	}
	order, err := market_order(ctx, trdStrategy.ConvMarket, BUY, qty, trd.stratId)
	update_trd_order(weightOrder, trd)
	if err != nil {
		return err
	}
	if order.Status == "REJECTED" {
		err := errors.New("Order was rejected.")
		return err
	}

	//Calc the price
	var orderPrice float64
	for _, fill := range order.Fills {
		orderPrice = orderPrice + (fill.Price * fill.Qty)
	}
	//Write result to analytics file. No need to wait before return.
	log.Add_analytics(order.StratID, order.Symbol, orderPrice, order.Qty)
	return nil
}

func parse_price(ctx context.Context, errCh chan error, resultCh chan float64, price float64, market string) error {
	doneCh := make(chan float64)
	go func() {
		//PRICE_FILTER
		minPrice := symbolsFilters[market]["PRICE_FILTER"].MinPrice
		maxPrice := symbolsFilters[market]["PRICE_FILTER"].MaxPrice
		precision := symbolsFilters[market]["PRICE_FILTER"].lotPrc
		if price < minPrice || price > maxPrice {
			err := errors.New("Price filter not respected.")
			errCh <- err
			return
		}
		if precision != 0 {
			price = round_down(price, precision)
		}

		//PERCENT_PRICE
		avgPrice, err := get_avg_price(ctx, market)
		if err != nil {
			errCh <- err
			return
		}
		multipDown := symbolsFilters[market]["PERCENT_PRICE"].MultipDown
		mulipUp := symbolsFilters[market]["PERCENT_PRICE"].MultipUp
		if price < avgPrice*multipDown || price > avgPrice*mulipUp {
			err := errors.New("AvgPrice filter not respected.")
			errCh <- err
			return
		}
		doneCh <- price
		return
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case price := <-doneCh:
		resultCh <- price
		return nil
	}
}

func parse_qty(ctx context.Context, errCh chan error, resultCh chan float64, qty float64, market string) {
	doneCh := make(chan float64)
	go func() {
		minQty := symbolsFilters[market]["LOT_SIZE"].MinQty
		maxQty := symbolsFilters[market]["LOT_SIZE"].MaxQty
		precision := symbolsFilters[market]["LOT_SIZE"].lotPrc
		if qty < minQty || qty > maxQty {
			err := errors.New("Lot filter not respected.")
			errCh <- err
			return
		}
		if precision != 0 {
			qty = round_down(qty, precision)
		}
		doneCh <- qty
	}()

	select {
	case <-ctx.Done():
		return
	case qty := <-doneCh:
		resultCh <- qty
		return
	}
}

func parse_market_qty(ctx context.Context, qty float64, market string) (float64, error) {
	//MARKET_LOT_SIZE filters
	minQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MinQty
	maxQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MaxQty
	precision := symbolsFilters[market]["MARKET_LOT_SIZE"].lotPrc
	//If filter isn't defined in MARKET_LOT_SIZE then use LOT_SIZE filter.
	if minQty == 0 {
		minQty = symbolsFilters[market]["LOT_SIZE"].MinQty
	}
	if maxQty == 0 {
		maxQty = symbolsFilters[market]["LOT_SIZE"].MaxQty
	}
	if precision == 0 {
		precision = symbolsFilters[market]["LOT_SIZE"].lotPrc
	}

	if qty < minQty || qty > maxQty {
		err := errors.New("Lot filter not respected.")
		return 0, err
	}
	if precision != 0 {
		qty = round_down(qty, precision)
	}
	return qty, nil
}

func parse_min_notional(ctx context.Context, price, qty float64, market string) error {
	minNotional := symbolsFilters[market]["MIN_NOTIONAL"].MinNotional
	maxNotional := symbolsFilters[market]["MIN_NOTIONAL"].MaxNotional
	notional := price * qty
	if maxNotional != 0 {
		if notional < minNotional || notional > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	} else {
		if notional < minNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	}
	return nil
}

func parse_market_min_notional(ctx context.Context, price, qty float64, market string) error {
	applays := symbolsFilters[market]["MIN_NOTIONAL"].ApplyToMarket
	minApplys := symbolsFilters[market]["MIN_NOTIONAL"].ApplyMinToM
	maxApplys := symbolsFilters[market]["MIN_NOTIONAL"].ApplyMaxToM
	minNotional := symbolsFilters[market]["MIN_NOTIONAL"].MinNotional
	maxNotional := symbolsFilters[market]["MIN_NOTIONAL"].MaxNotional
	notional := price * qty
	if applays == true {
		if notional < minNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	} else if minApplys == true {
		if maxApplys == true {
			if notional < minNotional && notional > maxNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		} else {
			if notional < minNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		}
	} else if maxApplys == true {
		if notional > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	}
	return nil
}

func retry_order(ctx context.Context, market trdMarket, trd *trdInfo) error {
	ctx = context.WithValue(ctx, ctxKey("reqWeight"), true)
	for {
		switch market {
		case SELL:
			err := sell_order(ctx, trd)
			if err == nil {
				return nil
			}
		case CONV:
			err := conv_order(ctx, trd)
			if err == nil {
				return nil
			}
		}
	}
}

func trd_signal() (bool, float64) {
	//TODO evtl. include sync.Waitgroup and goroutine
	buyPrice := buyMarketP.get_price()
	sellPrice := sellMarketP.get_price()
	convPrice := convMarketP.get_price()
	if m := sellPrice/convPrice - buyPrice; m > 0 {
		return true, buyPrice
	} else {
		return false, 0
	}
}

func update_trd_req(weight int, trd *trdInfo) {
	trd.reqWeight = trd.reqWeight - weight
	trd.rawReqs = trd.rawReqs - 1
}

func update_trd_order(weight int, trd *trdInfo) {
	trd.reqWeight = trd.reqWeight - weight
	trd.rawReqs = trd.rawReqs - 1
	trd.orders = trd.orders - 1
}

func unblock_limit_ctrs(trd *trdInfo) {
	exLimitsCtrs.reqWeight.increase_counter(trd.reqWeight)
	exLimitsCtrs.rawReq.increase_counter(trd.rawReqs)
	exLimitsCtrs.orders.increase_counter(trd.orders)
	exLimitsCtrs.maxOrders.increase_counter(trd.orders)
}

func round_down(n float64, precision int) float64 {
	base := []string{"1"}
	for i := 0; i < precision; i++ {
		base = append(base, "0")
	}
	roundString := strings.Join(base, "")
	roundDecimal, _ := strconv.Atoi(roundString)
	roundedN := math.Floor(n*float64(roundDecimal)) / float64(roundDecimal)
	return roundedN
}
