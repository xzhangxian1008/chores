package main

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestTmp(t *testing.T) {
	time.LoadLocation("Europe/Vilnius")
}

func truncate(f float64, dec int) float64 {
	shift := math.Pow10(dec)
	tmp := f * shift
	if math.IsInf(tmp, 0) {
		return f
	}
	if shift == 0.0 {
		if math.IsNaN(f) {
			return f
		}
		return 0.0
	}
	return math.Trunc(tmp) / shift
}

func ret() []int {
	fmt.Println("Run ret")
	return []int{1, 2, 3}
}

func TestM(t *testing.T) {
	for _, item := range ret() {
		fmt.Println(item)
	}
}
