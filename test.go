package main

import (
	"fmt"
	"math/big"
)

func main() {
	// result := "000000000000000000000000ac35beb095f49031139886dd20ad7a417a65bc89"
	// address := "41C1A74CD01732542093F5A87910A398AD70F04BD7"
	// address = address[2:]

	// data := xstrings.RightJustify(address, 64, "0")

	// log.Println(strings.ToLower(data))

	s := "0000000000000000000000000000000000000000000000000000ecf8e015bd05"
	i := new(big.Int)
	i.SetString(s, 16)
	fmt.Println(i) // 10

}
