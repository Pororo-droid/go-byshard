package main

import "fmt"

// Example usage
func main() {
	var c chan int
	c = make(chan int, 2)

	c <- 1
	c <- 2

	fmt.Printf("%d\n", <-c)
	fmt.Printf("%d\n", <-c)
}
