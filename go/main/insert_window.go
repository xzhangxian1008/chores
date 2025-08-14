package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
)

const tableName = "agg_test"

func getWindowDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", buildDSN("10.2.12.124", "root", "7001", "test"))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func generateRandomWindowString(length int) string {
	asciiChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result []rune
	for i := 0; i < length; i++ {
		result = append(result, rune(asciiChars[rand.Intn(len(asciiChars))]))
	}
	return fmt.Sprintf("'%s'", string(result))
}

func runWindowWorker(db *sql.DB, tableName string, id int) {
	totalRowNum := int(rand.Int31n(500000) + 500000)

	// Generate partition
	partitions := make([]int, 0)
	totalAppendedRowNum := 0
	for totalAppendedRowNum < totalRowNum {
		rowNum := int(rand.Int31n(200000) + 1)

		if totalAppendedRowNum+rowNum > totalRowNum {
			rowNum = totalRowNum - totalAppendedRowNum
			partitions = append(partitions, rowNum)
		} else {
			partitions = append(partitions, rowNum)
		}
		totalAppendedRowNum += rowNum
	}

	insertedRowNum := 0

	partitionValue := 10000 * id
	for _, partitionRowNUm := range partitions {
		insertSQL := ""
		partitionValue++
		appendedValueCount := 1

		// values of column o2 may be duplicate, so we need to record it in another variable
		o2Value := 0
		insertSQL = fmt.Sprintf("insert into %s values (%d, 0, %d, null, null, null, null)", tableName, partitionValue, o2Value)
		for i := 1; i < partitionRowNUm; i++ {
			if rand.Intn(10) >= 8 {
				o2Value++
			}

			if appendedValueCount > 1000 {
				_, err := db.Exec(insertSQL)
				if err != nil {
					panic(fmt.Sprintf("%v", err))
				}
				insertSQL = fmt.Sprintf("insert into %s values (%d, %d, %d, null, null, null, null)", tableName, partitionValue, i, o2Value)
				appendedValueCount = 1
				continue
			}

			if rand.Intn(100) < 10 {
				insertSQL = fmt.Sprintf("%s, (%d, %d, %d, null, null, null, null)", insertSQL, partitionValue, i, o2Value)
			} else {
				insertSQL = fmt.Sprintf("%s, (%d, %d, %d, %d, %.2f, %s, %s)", insertSQL, partitionValue, i, o2Value, rand.Intn(100000), rand.Float32()+float32(rand.Intn(100)), generateRandomWindowString(rand.Intn(100)), generateRandomDatetime())
			}
			appendedValueCount++
			insertedRowNum++

			if insertedRowNum%10000 == 0 {
				fmt.Printf("%d insert %d\n", id, insertedRowNum)
			}
		}

		// Insert remaining data
		_, err := db.Exec(insertSQL)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
	}
}

func prepareData() {
	db, err := getWindowDB()
	if err != nil {
		panic("Fail to get db")
	}

	db.Exec(fmt.Sprintf("drop table if exists %s", tableName))
	db.Exec(fmt.Sprintf("create table %s (p int,o1 int,o2 int,v_int int, v_decimal decimal(15, 3),v_str varchar(100),v_datetime datetime);", tableName))

	fmt.Println("Schema done")

	workerNum := 5
	waiter := &sync.WaitGroup{}
	waiter.Add(workerNum)
	for i := 0; i < workerNum; i++ {
		go func(idx int) {
			defer waiter.Done()
			runWindowWorker(db, tableName, idx)
		}(i)
	}
	waiter.Wait()

	_, err = db.Exec("alter table test.agg_test set tiflash replica 1")
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}
}
