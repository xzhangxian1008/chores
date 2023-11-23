package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var lock sync.RWMutex
var syncer chan struct{}
var syncerExit chan struct{}
var mu sync.Mutex

const readNum = 100
const writeNum = 0

func read() {
	<-syncer
	mu := sync.Mutex{}
	num := int64(300)
	totalTime := int64(0)
	for i := int64(0); i < num; i++ {
		start := time.Now()
		mu.Lock()
		waitTime := int64(time.Since(start))
		totalTime += waitTime
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		mu.Unlock()
	}
	mu.Lock()
	fmt.Printf("read avg: %dns\n", totalTime/num/int64(time.Nanosecond))
	mu.Unlock()
	syncerExit <- struct{}{}
}

func write() {
	<-syncer
	num := int64(300)
	totalTime := int64(0)
	for i := int64(0); i < num; i++ {
		start := time.Now()
		lock.Lock()
		waitTime := int64(time.Since(start))
		totalTime += waitTime
		lock.Unlock()
	}
	mu.Lock()
	fmt.Printf("write avg: %dus\n", totalTime/num/int64(time.Microsecond))
	mu.Unlock()
	syncerExit <- struct{}{}
}

func f() {
	defer fmt.Println(1)
	defer fmt.Println(2)
	fmt.Println("exit...")
}

// 26326 427095560
// 427098424 855233053
// 855246475 1289258768
// 1289269338 1720523486
// 1720524265 2147481675

func main() {
	num := 20000000
	rangeNum := 5
	// avgAmount := num / rangeNum
	// data := make([]int32, 0, num)
	rangeData := make([][]int32, rangeNum)
	for i := 0; i < num; i++ {
		randNum := rand.Int31()
		if randNum <= 427095560 {
			rangeData[0] = append(rangeData[0], randNum)
		} else if randNum <= 855233053 {
			rangeData[1] = append(rangeData[1], randNum)
		} else if randNum <= 1289258768 {
			rangeData[2] = append(rangeData[2], randNum)
		} else if randNum <= 1720523486 {
			rangeData[3] = append(rangeData[3], randNum)
		} else {
			rangeData[4] = append(rangeData[4], randNum)
		}
	}

	// sort.Slice(data, func(i, j int) bool { return data[i] < data[j] })

	// for _, d := range data {
	// 	fmt.Println(d)
	// }

	// for i := 0; i < rangeNum; i++ {
	// 	fmt.Printf("%d %d\n", data[i*avgAmount], data[(i+1)*avgAmount-1])
	// }

	for _, nums := range rangeData {
		fmt.Println(len(nums))
	}
}
