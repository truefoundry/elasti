package main

import "fmt"

func main() {
	b := "then"
	a := func() {
		b = "when"
	}
	a()
	fmt.Println(b)
}
