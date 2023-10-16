package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

type Opaque struct {
	a int
}

func check(err error) {
	if err != nil {
		panic("")
	}
}

func testWorkerAsync(file io.ReaderAt) []byte {
	fileSize := 1000000
	r := io.NewSectionReader(file, 0, int64(fileSize))
	bufio.NewReader(r)

	cellSize := 500
	res := make([]byte, 0)
	formatCh := make(chan []byte, 3000)
	go func() {
		defer close(formatCh)
		for consumedSize := 0; consumedSize < fileSize; consumedSize += cellSize {
			cell := make([]byte, cellSize)
			io.ReadFull(r, cell)
			formatCh <- cell
		}
	}()

	for format := range formatCh {
		res = append(res, format...)
	}

	return res
}

func testWorkerBlocked(file io.ReaderAt) []byte {
	fileSize := 1000000
	r := io.NewSectionReader(file, 0, int64(fileSize))
	bufio.NewReader(r)

	cellSize := 500
	res := make([]byte, 0)
	for consumedSize := 0; consumedSize < fileSize; consumedSize += cellSize {
		cell := make([]byte, cellSize)
		io.ReadFull(r, cell)
		res = append(res, cell...)
	}
	return res
}

var loopNum int = 3000

func testAsync(file io.ReaderAt) {
	fmt.Println("--------Async--------")
	start := time.Now()
	totalLen := 0
	for i := 0; i < loopNum; i++ {
		res := testWorkerAsync(file)
		totalLen += len(res)
	}
	elapsed := time.Since(start)
	fmt.Println(totalLen)
	fmt.Println(elapsed)
	fmt.Println("---------------------")
}

func testBlocked(file io.ReaderAt) {
	fmt.Println("--------Blocked--------")
	start := time.Now()
	totalLen := 0
	for i := 0; i < loopNum; i++ {
		res := testWorkerBlocked(file)
		totalLen += len(res)
	}
	elapsed := time.Since(start)
	fmt.Println(totalLen)
	fmt.Println(elapsed)
	fmt.Println("-----------------------")
}

func main() {
	file, err := os.Open("tmp")
	check(err)
	testBlocked(file)
	testAsync(file)
}
