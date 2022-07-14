package binance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type WsStream struct {
	Stream string    `json:"stream"`
	Data   BookDepth `json:"data"`
}

type BookDepth struct {
	Bids [][]json.Number `json:"bids"`
	Asks [][]json.Number `json:"asks"`
}

type Order struct {
	Price string
	Qty   string
}

func handle_resp(msg []byte, markets TrdStradegy) {
	r := bytes.NewReader(msg)
	dec := json.NewDecoder(r)
	var book WsStream
	if dec.More() {
		dec.Decode(&book)
	}
	//fmt.Println(book)
	switch strings.TrimSuffix(book.Stream, "@depth10") {
	case markets.BuyMarket:
		buy_handler(book.Data.Asks)
	}
}

func buy_handler(market [][]json.Number) {
	fmt.Println(market)
	fmt.Printf("HERE I AM %s", market[2])
}
