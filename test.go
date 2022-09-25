package main

import (
	"fmt"
)

func main() {
	if b, f := test(); b == true {
		fmt.Println(b, f)
	}
}

func test() (bool, float64) {
	return true, 10.5
}
