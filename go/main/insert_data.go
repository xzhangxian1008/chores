package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

const totalRowNum = 500000
const oneBatchRowNum = 1000
const table1Name = "ct1"
const table2Name = "ct2"
const addr = "10.2.12.124"
const port = "7001"
const user = "root"
const dbName = "test"

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

func generateRandomString() string {
	if rand.Intn(10000) == 5000 {
		return "null"
	}
	asciiChars := "abcde"
	var result []rune
	for i := 0; i < 50; i++ {
		result = append(result, rune(asciiChars[rand.Intn(len(asciiChars))]))
	}
	return fmt.Sprintf("'%s'", string(result))
}

func buildDSN(addr string, user string, port, db string, params ...string) string {
	// Commonly, the password is always empty in my development environment
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, "", addr, port, db)
	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}
	return dsn
}

func getDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", buildDSN(addr, user, port, dbName))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func runWorker(thdID int, rowNum int) {
	fmt.Printf("Thread %d is running...\n", thdID)
	keyID := thdID * rowNum

	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	insertT1SQL := ""
	insertT2SQL := ""
	appendedRowCount := 0
	for i := 0; i < rowNum; i++ {
		if len(insertT1SQL) == 0 {
			insertT1SQL = fmt.Sprintf("insert into %s values (%d, %s, %s, %s, %s)", table1Name, keyID, generateRandomInt(), generateRandomFloat(), generateRandomString(), generateRandomDatetime())
			insertT2SQL = fmt.Sprintf("insert into %s values (%d, %s, %s, %s, %s)", table2Name, keyID, generateRandomInt(), generateRandomFloat(), generateRandomString(), generateRandomDatetime())
		} else {
			insertT1SQL = fmt.Sprintf("%s, (%d, %s, %s, %s, %s)", insertT1SQL, keyID, generateRandomInt(), generateRandomFloat(), generateRandomString(), generateRandomDatetime())
			insertT2SQL = fmt.Sprintf("%s, (%d, %s, %s, %s, %s)", insertT2SQL, keyID, generateRandomInt(), generateRandomFloat(), generateRandomString(), generateRandomDatetime())
		}

		appendedRowCount++
		keyID++

		if i%10000 == 0 {
			fmt.Printf("Thread %d inserts %d rows\n", thdID, i)
		}

		if len(insertT1SQL) >= oneBatchRowNum {
			_, err := db.Exec(insertT1SQL)
			if err != nil {
				panic(fmt.Sprintf("Fail to insert: %s, error: %v", insertT1SQL, err))
			}
			_, err = db.Exec(insertT2SQL)
			if err != nil {
				panic(fmt.Sprintf("Fail to insert: %s, error: %v", insertT2SQL, err))
			}
			insertT1SQL = ""
			insertT2SQL = ""
			appendedRowCount = 0
		}
	}

	if len(insertT1SQL) != 0 {
		_, err := db.Exec(insertT1SQL)
		if err != nil {
			panic(fmt.Sprintf("Fail to insert: %s, error: %v", insertT1SQL, err))
		}
		_, err = db.Exec(insertT2SQL)
		if err != nil {
			panic(fmt.Sprintf("Fail to insert: %s, error: %v", insertT2SQL, err))
		}
	}

}

func run() {
	wg := &sync.WaitGroup{}
	threadNum := 5
	wg.Add(threadNum)
	for i := range threadNum {
		go func(i int) {
			runWorker(i, totalRowNum/threadNum)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
