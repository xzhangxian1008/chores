package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
)

func Round(f float64, n int) float64 {
	pow10 := math.Pow10(n)
	return math.Round(f*pow10) / pow10
}

func generateRandomInt() string {
	if rand.Intn(10000) == 5000 {
		return "null"
	}
	return strconv.Itoa(rand.Intn(1000000))
}

func generateRandomDatetime() string {
	if rand.Intn(10000) == 5000 {
		return "null"
	}
	return fmt.Sprintf("'%d-%d-%d %d:0:0'", rand.Intn(500)+2000, rand.Intn(12)+1, rand.Intn(25)+1, rand.Intn(6))
}

func generateRandomFloat() string {
	if rand.Intn(10000) == 5000 {
		return "null"
	}
	return fmt.Sprintf("%.2f", rand.Float32()+float32(rand.Intn(1000000)))
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
