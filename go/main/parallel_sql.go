package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

const totalRowNum = 500000
const oneBatchRowNum = 1000

type RuntimeInfo struct {
	threadID int
	db       *sql.DB
}

type InsertSqlGenerateInfo struct {
	totalRowCnt     int
	generatedRowCnt int
	oneBatchCnt     int
	tableName       string
}

type Result struct {
	qps float64
}

// Return true if more sqls will be generated
func generateInsertSql(info *InsertSqlGenerateInfo) (string, bool) {
	sql := fmt.Sprintf("insert into %s values (%d, %d)", info.tableName, rand.Intn(99999999), rand.Intn(99999999))
	info.generatedRowCnt++

	for i := 1; (i < info.oneBatchCnt) && (info.generatedRowCnt < info.totalRowCnt); i, info.generatedRowCnt = i+1, info.generatedRowCnt+1 {
		sql = fmt.Sprintf("%s, (%d, %d)", sql, rand.Intn(99999999), rand.Intn(99999999))
	}
	return sql, info.generatedRowCnt < info.totalRowCnt
}

func runTask(_ int, task func(RuntimeInfo) bool) {
	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	defer func() {
		db.Close()
	}()

	for task(RuntimeInfo{db: db}) {
	}
}

func runImpl(threadCnt int, task func(RuntimeInfo) bool) (qps Result) {
	wg := &sync.WaitGroup{}

	wg.Add(threadCnt)
	for i := range threadCnt {
		go func(i int) {
			runTask(i, task)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return Result{}
}

func run() {
	// Return true if we need to run this task again
	task := func(info RuntimeInfo) bool {
		fmt.Println("Task begin to run...")
		// Try(info.db.Exec("begin pessimistic"))
		insertInfo := InsertSqlGenerateInfo{totalRowCnt: 1000000000, oneBatchCnt: 3000, tableName: "t_base"}
		var insertSQL string
		var more bool
		for {
			insertSQL, more = generateInsertSql(&insertInfo)
			Try(info.db.Exec(insertSQL))
			if !more {
				break
			}
			fmt.Printf("%d have been inserted\n", insertInfo.generatedRowCnt)
		}
		// Try(info.db.Exec("commit"))
		fmt.Println("Task done")
		return false
	}

	runImpl(30, task)
}
