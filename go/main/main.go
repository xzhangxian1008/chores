package main

import (
	"fmt"
)

func f() (err error) {
	return fmt.Errorf("123")
}

func main() {
	run()
}
