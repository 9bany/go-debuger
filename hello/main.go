package main

import "fmt"

func main() {
	h := f1(0x11111111)
	fmt.Printf("hello! f1 returned: %X\n\n", h)
}

func f1(i int) int {
	return f2(i, 0x22222222)
}

func f2(i, j int) int {
	var k int
	k = f3(i + j)
	return k
}

func f3(i int) int {
	var j int
	j = 0x44444444
	return i + j
}
