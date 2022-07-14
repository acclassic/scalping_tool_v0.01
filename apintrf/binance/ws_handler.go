package binance

import (
	"encoding/json"
	"fmt"
)

func handle_resp(msg *WsStream, markets TrdStradegy) {
}

func buy_handler(market [][]json.Number) {
	fmt.Println(market)
	fmt.Printf("HERE I AM %s", market[2])
}
