package binance

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

//TODO check if possible to only get avgPrice once and pass to other funcs.
//TODO check if chan needs init func for variable buffer size
var trdCh = make(chan bool, 3)

type trdInfo struct {
	buyPrice float64
	sellAmnt float64
	convAmnt float64
}

func trd_handler(ctx context.Context) {
	for {
		select {
		case trdCh <- true:
			//TODO implement exLimitsCtrs
			var trd *trdInfo
			err := buy_order(ctx, trd)
			if err != nil {
				<-trdCh
			}

			//TODO check the loop again. This way it will free the chan on the first itiration
			// If order can't be sold retry 3 times. If sold continue to conv_order or drop trd and free chan.
			err = sell_order(ctx, trd)
			if err != nil {
				err := retry_order(3, sell_order(ctx, trd), trdStrategy.SellMarket)
				if err != nil {
					//TODO log err
					<-trdCh
				}
			}

			// If order can't be sold retry 3 times. If sold continue to log and end trd or drop trd and free chan.
			err = conv_order(ctx, trd)
			if err != nil {
				err := retry_order(3, conv_order(ctx, trd), trdStrategy.ConvMarket)
				if err != nil {
					//TODO log err
					<-trdCh
				}
			}

			<-trdCh
		default:
			//If trdCh is full drop chan
			return
		}
	}
}

//TODO check this on testnet
func buy_order(ctx context.Context, trd *trdInfo) error {
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, ctxKey("origin"), "buyOrder")
	mPrice := buyMarketP.get_price()
	price := trdFunds.get_funds(trdStrategy.TrdRate)
	qty := price / mPrice

	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
	priceCh := make(chan float64)
	qtyCh := make(chan float64)
	errCh := make(chan error)
	go parse_price(ctx, errCh, priceCh, price, trdStrategy.BuyMarket)
	go parse_qty(ctx, errCh, qtyCh, qty, trdStrategy.BuyMarket)
	select {
	case price <- priceCh:
		select {
		case qty <- qtyCh:
			err := parse_min_notional(ctx, price, qty)
			if err != nil {
				return err
			}
			order := limit_order(ctx, trdStrategy.BuyMarket, BUY, price, qty)
			if order.Status == "REJECTED" {
				err := errors.New("Order was rejected.")
				return err
			}
			trd.buyPrice = price
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
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, ctxKey("origin"), "sellOrder")
	qty := trd.sellAmnt

	//Pass value and error chan then wait for value return. No need to check qtyCh because both values are needed to continue. If err occurs drop trd and trdCh.
	qtyCh := make(chan float64)
	errCh := make(chan error)
	go parse_market_qty(ctx, errCh, qtyCh, qty, trdStrategy.SellMarket)
	select {
	case qty <- qtyCh:
		price := get_avg_price(ctx, trdStrategy.SellMarket)
		err := parse_market_min_notional(ctx, price, qty)
		if err != nil {
			return err
		}
		order := market_order(ctx, trdStrategy.SellMarket, SELL, qty)
		if order.Status == "REJECTED" {
			err := errors.New("Order was rejected.")
			return err
		}
		trd.convAmnt = order.Qty
		return nil
	case err := <-errCh:
		cancel()
		return err
	}
}

func conv_order(ctx context.Context, trd *trdInfo) error {
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, ctxKey("origin"), "convOrder")
	qty := trd.convAmnt

	//No need to check price or lot filter because by using quoteOrderQty the lot filter is respected.
	price := get_avg_price(ctx, trdStrategy.ConvMarket)
	err := parse_market_min_notional(ctx, price, qty)
	if err != nil {
		return err
	}
	order := market_order(ctx, trdStrategy.ConvMarket, BUY, qty)
	if order.Status == "REJECTED" {
		err := errors.New("Order was rejected.")
		return err
	}
	return nil
}

//TODO implement ctx. Cancel ctx on err
func parse_price(ctx context.Context, errCh chan error, resultCh chan float64, price float64, market string) {
	doneCh := make(chan bool)
	go func() {
		//PRICE_FILTER
		minPrice := symbolsFilters[market]["PRICE_FILTER"].MinPrice
		maxPrice := symbolsFilters[market]["PRICE_FILTER"].MaxPrice
		precision := symbolsFilters[market]["PRICE_FILTER"].precision
		if price < minPrice || price > maxPrice {
			err := errors.New("Price filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		price, _ := strconv.ParseFloat(fmt.Sprintf(fmtcmd, price), 64)

		//PERCENT_PRICE
		avgPrice := get_avg_price(ctx)
		multipDown := symbolsFilters[market]["PERCENT_PRICE"].MultipDown
		mulipUp := symbolsFilters[market]["PERCENT_PRICE"].MultipUp
		if price < avgPrice*multipDown || price > avgPrice*mulipUp {
			err := errors.New("AvgPrice filter not respected.")
			errCh <- err
			return
		}
		doneCh <- true
		return
	}()

	select {
	case <-ctx.Done():
		return
	case <-doneCh:
		resultCh <- price
		return
	}
}

func parse_qty(ctx context.Context, errCh chan error, resultCh chan float64, qty float64, market string) {
	doneCh := make(chan bool)
	go func() {
		minQty := symbolsFilters[market]["LOT_SIZE"].MinQty
		maxQty := symbolsFilters[market]["LOT_SIZE"].MaxQty
		precision := symbolsFilters[market]["LOT_SIZE"].lotPrc
		if qty < minQty || qty > maxQty {
			err := errors.New("Lot filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		qty, _ := strconv.ParseFloat(fmt.Sprintf(fmtcmd, qty), 64)
		doneCh <- true
	}()

	select {
	case <-ctx.Done():
		return
	case <-doneCh:
		resultCh <- qty
		return
	}
}

func parse_market_qty(ctx context.Context, errCh chan error, resultCh chan float64, qty float64, market string) {
	doneCh := make(chan bool)
	go func() {
		minQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MinQty
		maxQty := symbolsFilters[market]["MARKET_LOT_SIZE"].MaxQty
		precision := symbolsFilters[market]["MARKET_LOT_SIZE"].mLotPrc
		if qty < minQty || qty > maxQty {
			err := errors.New("Lot filter not respected.")
			errCh <- err
			return
		}
		fmtCmd := fmt.Sprint("%.", precision, "f")
		qty, _ := strconv.ParseFloat(fmt.Sprintf(fmtcmd, qty), 64)
		doneCh <- true
	}()

	select {
	case <-ctx.Done():
		return
	case <-doneCh:
		resultCh <- qty
		return
	}
}

func parse_min_notional(ctx context.Context, price, qty float64, market string) error {
	minNotional := symbolsFilters[market]["MIN_NOTIONAL"].MinNotional
	maxNotional := symbolsFilters[market]["MIN_NOTIONAL"].MaxNotional
	if maxNotional != 0 {
		if price*qty < minNotional || price*qty > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	} else {
		if price*qty < minNotional {
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
	} else if minNotional == true {
		if maxNotional == true {
			if price*qty < minNotional || price*qty > maxNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		} else {
			if price*qty < minNotional {
				err := errors.New("Min Notional filter not respected")
				return err
			}
		}
	} else if maxNotional == true {
		if price*qty > maxNotional {
			err := errors.New("Min Notional filter not respected")
			return err
		}
	}
	return nil
}

func retry_order(n int, f func(ctx, trd *trdInfo), market string) err {
	for i := 0; i < n; i++ {
		err := f(ctx, trd)
		if err == nil {
			return nil
		}
	}
	err := fmt.Errorf("Retried to execute order %d time failed. Order info: %s. Market: %s.", n, trd, market)
	return err
}