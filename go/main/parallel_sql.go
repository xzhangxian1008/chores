package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const totalRowNum = 500000
const oneBatchRowNum = 1000

// const table1Name = "p1000"
// const table2Name = "ct2"

func runInsertWorker(thdID int, rowNum int) {
	fmt.Printf("Thread %d is running...\n", thdID)
	keyID := thdID * rowNum

	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	insertT1SQL := ""
	appendedRowCount := 0
	for i := 0; i < rowNum; i++ {
		if len(insertT1SQL) == 0 {
			insertT1SQL = fmt.Sprintf("insert into %s values (%s, '%s')", table1Name, generateRandomInt(), generateRandomString(rand.Intn(20)))
		} else {
			insertT1SQL = fmt.Sprintf("%s, (%s, '%s')", insertT1SQL, generateRandomInt(), generateRandomString(rand.Intn(20)))
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

			insertT1SQL = ""
			appendedRowCount = 0
		}
	}

	if len(insertT1SQL) != 0 {
		_, err := db.Exec(insertT1SQL)
		if err != nil {
			panic(fmt.Sprintf("Fail to insert: %s, error: %v", insertT1SQL, err))
		}
	}
}

func runWorkerForTCMSImpl() {
	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	defer func() {
		db.Close()
	}()

	sql := "select group_concat(distinct col0, col1) from t6 group by col2 limit 30;"

	ret, err := query(db, sql, " ")
	if err != nil {
		panic(err)
	}

	for _, res := range ret {
		fmt.Println(res)
	}

	for i, res := range ret {
		strs := strings.Split(res, ",")
		sort.Slice(strs, func(i, j int) bool {
			return strs[i] < strs[j]
		})
		newResStr := ""
		for _, item := range strs {
			newResStr += item
		}
		ret[i] = newResStr
	}

	fmt.Println("----------------")

	for _, res := range ret {
		fmt.Println(res)
	}
}

func runWorkerForTCMS(threadNum int) {
	wg := &sync.WaitGroup{}
	wg.Add(threadNum)
	for i := range threadNum {
		go func(i int) {
			runWorkerForTCMSImpl()
			wg.Done()
		}(i)
	}
	wg.Wait()
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

	_, err = db.Exec("set tidb_mem_quota_query=20737418240")
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
	fmt.Printf("elapsed: %f\n", elapsed)
	return float64(totalExecutedNum.Load()) / elapsed
}

func run() {
	variables := []string{"optimized"}
	// variables := []string{"optimized", "legacy"}
	rowNums := []int{1000}
	// rowNums := []int{100, 500, 1000}
	// threadNums := []int{50, 100, 400}
	threadNums := []int{20}
	totalQueryNum := 10000
	repeatNum := 1000
	for _, variable := range variables {
		fmt.Printf("----------Hash Join Type: %s----------\n", variable)
		for _, rowNum := range rowNums {
			sql := "explain analyze select /*+ HASH_JOIN_BUILD(b1000) */ * from p1000 join b1000 on p1000.col0 = b1000.col0 and p1000.col1 = b1000.col1;"
			// sql := "explain analyze SELECT /*+ HASH_JOIN(lineitem) */ COUNT(*) FROM orders JOIN lineitem ON l_orderkey = o_orderkey;"
			for _, threadNum := range threadNums {
				totalQPS := float64(0)
				for range repeatNum {
					qps := runImpl(threadNum, int64(totalQueryNum/threadNum), sql, variable)
					totalQPS += qps
				}

				fmt.Printf("var: %s, row num: %d, thread_num: %d, qps: %.2f\n", variable, rowNum, threadNum, totalQPS/float64(repeatNum))
			}
		}
	}
}
