package binance

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	buyPrice  float64
	sellAmnt  float64
	convAmnt  float64
	stratId   int
	reqWeight int
	rawReqs   int
	orders    int
}

type orderParams struct {
	price float64
	qty   float64
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
					err := retry_order(ctx, 3, SELL, &trd)
					if err != nil {
						unblock_limit_ctrs(&trd)
						log.Strat_logger().Println(err)
						<-trdCh
					}
				}

				// If order can't be sold retry 3 times. If sold continue to log and end trd or drop trd and free chan.
				err = conv_order(ctx, &trd)
				if err != nil {
					err := retry_order(ctx, 3, CONV, &trd)
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

//TODO check this on testnet
func buy_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(ctx)
	mPrice := buyMarketP.get_price()
	var ordParams orderParams
	ordParams.price = 70
	//ordParams.price = trdFunds.get_funds(trdStrategy.TrdRate)
	ordParams.qty = ordParams.price / mPrice

	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
	priceCh := make(chan float64)
	qtyCh := make(chan float64)
	errCh := make(chan error)
	go parse_price(ctx, errCh, priceCh, ordParams, trdStrategy.BuyMarket)
	go parse_qty(ctx, errCh, qtyCh, ordParams, trdStrategy.BuyMarket)
	select {
	case price := <-priceCh:
		select {
		case qty := <-qtyCh:
			err := parse_min_notional(ctx, ordParams, trdStrategy.BuyMarket)
			if err != nil {
				return err
			}
			order, err := limit_order(ctx, trdStrategy.BuyMarket, BUY, price, qty, trd.stratId)
			update_trd_order(weightOrder, trd)
			if err != nil {
				return err
			}
			if order.Status == "REJECTED" {
				err := errors.New("Order was rejected.")
				return err
			}
			//Write result to analytics file. No need to wait before return
			go log.Add_analytics(order.StratID, order.Symbol, order.Price, order.Qty)
			trd.buyPrice = ordParams.price
			trd.sellAmnt = order.Qty
			return nil
		case err := <-errCh:
			cancel()
			return err
		}
	case err := <-errCh:
		cancel()
		return err
	}
}

func sell_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(ctx)
	qty := trd.sellAmnt

	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
	qtyCh := make(chan float64)
	errCh := make(chan error)
	go parse_market_qty(ctx, errCh, qtyCh, qty, trdStrategy.SellMarket)
	select {
	case qty := <-qtyCh:
		price, err := get_avg_price(ctx, trdStrategy.SellMarket)
		update_trd_req(weightAvgPrice, trd)
		if err != nil {
			return err
		}
		err = parse_market_min_notional(ctx, price, qty, trdStrategy.SellMarket)
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
		//Sum amount of order
		var convAmnt float64
		for _, v := range order.Fills {
			convAmnt = convAmnt + v.Price
		}
		trd.convAmnt = convAmnt
		//Write result to analytics file. No need to wait before return.
		go log.Add_analytics(order.StratID, order.Symbol, convAmnt, order.Qty)
		return nil
	case err := <-errCh:
		cancel()
		return err
	}
}

func conv_order(ctx context.Context, trd *trdInfo) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	qty := trd.convAmnt

	//No need to check price or lot filter because by using quoteOrderQty the lot filter is respected.
	price, err := get_avg_price(ctx, trdStrategy.ConvMarket)
	update_trd_req(weightAvgPrice, trd)
	if err != nil {
		return err
	}
	err = parse_market_min_notional(ctx, price, qty, trdStrategy.ConvMarket)
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
	//Sum amount of order
	var convPrice float64
	for _, v := range order.Fills {
		convPrice = convPrice + v.Price
	}
	//Write result to analytics file. No need to wait before return.
	go log.Add_analytics(order.StratID, order.Symbol, convPrice, order.Qty)
	return nil
}

func parse_price(ctx context.Context, errCh chan error, resultCh chan float64, ordParams orderParams, market string) error {
	doneCh := make(chan float64)
	go func() {
		//PRICE_FILTER
		minPrice := symbolsFilters[market]["PRICE_FILTER"].MinPrice
		maxPrice := symbolsFilters[market]["PRICE_FILTER"].MaxPrice
		precision := symbolsFilters[market]["PRICE_FILTER"].lotPrc
		if ordParams.price < minPrice || ordParams.price > maxPrice {
			err := errors.New("Price filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		price, _ := strconv.ParseFloat(fmt.Sprintf(fmtCmd, ordParams.price), 64)

		//PERCENT_PRICE
		avgPrice, err := get_avg_price(ctx, market)
		if err != nil {
			errCh <- err
			return
		}
		multipDown := symbolsFilters[market]["PERCENT_PRICE"].MultipDown
		mulipUp := symbolsFilters[market]["PERCENT_PRICE"].MultipUp
		if ordParams.price < avgPrice*multipDown || ordParams.price > avgPrice*mulipUp {
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

func parse_qty(ctx context.Context, errCh chan error, resultCh chan float64, ordParams orderParams, market string) {
	doneCh := make(chan float64)
	go func() {
		minQty := symbolsFilters[market]["LOT_SIZE"].MinQty
		maxQty := symbolsFilters[market]["LOT_SIZE"].MaxQty
		precision := symbolsFilters[market]["LOT_SIZE"].lotPrc
		if ordParams.qty < minQty || ordParams.qty > maxQty {
			err := errors.New("Lot filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		qty, _ := strconv.ParseFloat(fmt.Sprintf(fmtCmd, ordParams.qty), 64)
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

func parse_market_qty(ctx context.Context, errCh chan error, resultCh chan float64, qty float64, market string) {
	doneCh := make(chan float64)
	go func() {
		minQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MinQty
		maxQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MaxQty
		precision := symbolsFilters[market]["MARKET_LOT_SIZE"].lotPrc
		if qty < minQty || qty > maxQty {
			err := errors.New("Lot filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		qty, _ := strconv.ParseFloat(fmt.Sprintf(fmtCmd, qty), 64)
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

func parse_min_notional(ctx context.Context, ordParams orderParams, market string) error {
	minNotional := symbolsFilters[market]["MIN_NOTIONAL"].MinNotional
	maxNotional := symbolsFilters[market]["MIN_NOTIONAL"].MaxNotional
	if maxNotional != 0 {
		if ordParams.price*ordParams.qty < minNotional || ordParams.price*ordParams.qty > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	} else {
		if ordParams.price*ordParams.qty < minNotional {
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
	if applays == true {
		if price*qty < minNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	} else if minApplys == true {
		if maxApplys == true {
			if price*qty < minNotional && price*qty > maxNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		} else {
			if price*qty < minNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		}
	} else if maxApplys == true {
		if price*qty > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	}
	return nil
}

func retry_order(ctx context.Context, n int, market trdMarket, trd *trdInfo) error {
	ctx = context.WithValue(ctx, ctxKey("reqWeight"), true)
	var err error
	for i := 0; i < n; i++ {
		switch market {
		case SELL:
			err = sell_order(ctx, trd)
		case CONV:
			err = conv_order(ctx, trd)
		}
		if err == nil {
			return nil
		}
	}
	err = fmt.Errorf("Retried to execute order %d times failed. Order info: %s. Market: %s. Error: %s", n, trd, market, err)
	return err
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
