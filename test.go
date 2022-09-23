package main

import (
	"fmt"
	"strings"
)

func main() {
	f := "0.00100000"
	s := strings.SplitAfter(f, ".")
	for _, v := range s[1] {
		fmt.Printf("%[1]T, %[1]s\n", v)
	}
}
