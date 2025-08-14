package main

import (
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const totalRowNum = 500000
const oneBatchRowNum = 1000
const table1Name = "ct1"
const table2Name = "ct2"
const addr = "10.2.12.57"
const port = "32651"
const user = "root"
const dbName = "test"

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

func query(db *sql.DB, q string, splitter string) ([]string, int, error) {
	rowValues := make([]string, 0)
	count := 0
	rows, err := db.Query(q)
	if err != nil {
		return rowValues, count, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return rowValues, count, err
	}
	values := make([]interface{}, len(cols))
	results := make([]sql.NullString, len(cols))
	for i := range values {
		values[i] = &results[i]
	}
	for rows.Next() {
		if err := rows.Scan(values...); err != nil {
			return rowValues, count, err
		}
		allFields := ""
		for _, v := range results {
			if !v.Valid {
				allFields += "NULL"
				continue
			}
			allFields += v.String
			allFields += splitter
		}
		rowValues = append(rowValues, allFields)
	}
	if err := rows.Err(); err != nil {
		return rowValues, count, err
	}
	return rowValues, count, nil
}

func runInsertWorker(thdID int, rowNum int) {
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

func runQueryWorker(thdID int, count *atomic.Int64, query_num int64, sql string, variable string) {
	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	defer func() {
		db.Close()
	}()

	_, err = db.Exec(fmt.Sprintf("set tidb_hash_join_version=%s", variable))
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}

	for i := 0; i < int(query_num); i++ {
		_, err := db.Exec(sql)
		if err != nil {
			panic(fmt.Sprintf("error: %v", err))
		}
		count.Add(1)
	}
}

func runImpl(threadNum int, query_num int64, sql string, variable string) (qps float64) {
	totalExecutedNum := atomic.Int64{}
	wg := &sync.WaitGroup{}

	wg.Add(threadNum)
	start := time.Now()
	for i := range threadNum {
		go func(i int) {
			runQueryWorker(i, &totalExecutedNum, int64(query_num), sql, variable)
			wg.Done()
		}(i)
	}
	wg.Wait()

	elapsed := Round(time.Since(start).Seconds(), 2)
	return float64(totalExecutedNum.Load()) / elapsed
}

func run() {
	variables := []string{"legacy", "optimized"}
	buildTableNames := []string{"t10", "t30", "t100", "t300", "t1000"}
	threadNums := []int{20, 50, 100, 200, 400}
	totalQueryNum := 20000
	repeatNum := 10
	for _, variable := range variables {
		fmt.Printf("----------Hash Join Type: %s----------\n", variable)
		for _, tableName := range buildTableNames {
			sql := fmt.Sprintf("explain analyze select /*+ HASH_JOIN_BUILD(%s) */ * from probe_t join %s on %s.col0 = probe_t.col0 and %s.col1 = probe_t.col1;", tableName, tableName, tableName, tableName)
			for _, threadNum := range threadNums {
				maxQPS := float64(0)
				for range repeatNum {
					qps := runImpl(threadNum, int64(totalQueryNum/threadNum), sql, variable)
					maxQPS = math.Max(qps, maxQPS)
				}

				fmt.Printf("var: %s, thread_num: %d, build table name: %s, qps: %.2f\n", variable, threadNum, tableName, maxQPS)
			}
		}
	}
}
