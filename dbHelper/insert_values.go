package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
)

// ---------------------- COMPARE CONFIG ----------------------

var (
	insertDBconfig = dbConfig{
		address: "10.2.12.124",
		port:    "8001",
		user:    "root",
		dbName:  "distinct_test",
		params:  make([]string, 0),
	}
)

// ------------------------------------------------------------

var config = insertConfig{}

const (
	intType = iota
	stringType
	decimalType
	floatType
	timeType
	dateType
	datetimeType
)

type colInfo struct {
	tp int

	// This is the filed for nvd. When nvd > 0, we will generate values in advance and
	// store them in this slice.
	// If len(values) is 0, it means that nvd is equal to 0
	values []string

	// -1 means that null is forbidden for this col
	// 60 means that we have 60% to generate null value for this column
	nullProbability int

	// This two fields is useful only when column type is string
	minStrLen int
	maxStrLen int
}

type insertConfig struct {
	totalRowCnt int
	batchSize   int
	tableName   string
	colInfos    []colInfo

	db *sql.DB
}

type insertTask struct {
	config    insertConfig
	taskID    int
	insertCnt int
}

func generateOneColValue(col colInfo) string {
	if col.nullProbability > 0 && rand.Intn(100) < col.nullProbability {
		return "null"
	}

	if len(col.values) > 0 {
		return col.values[rand.Intn(len(col.values))]
	}

	switch col.tp {
	case intType:
		return strconv.Itoa(rand.Int())
	case stringType:
		return generateRandomString(col.minStrLen, col.maxStrLen)
	case decimalType, floatType:
		return generateRandomFloat()
	case timeType:
		return generateRandomTime()
	case dateType:
		return generateRandomDate()
	case datetimeType:
		return generateRandomDatetime()
	}

	panic(fmt.Sprintf("Unrecognized type %d", col.tp))
}

func generateOneRow(colInfos []colInfo) string {
	row := ""
	for i, col := range colInfos {
		val := generateOneColValue(col)
		if i == 0 {
			row = val
		} else {
			row = fmt.Sprintf("%s, %s", row, val)
		}
	}

	return fmt.Sprintf("(%s)", row)
}

func startToInsertForOneTask(task insertTask) {
	db := task.config.db
	batchSize := task.config.batchSize
	tableName := task.config.tableName
	colInfos := task.config.colInfos

	insertSQL := ""
	rowCntOneSql := 0
	for range task.insertCnt {
		if len(insertSQL) == 0 {
			insertSQL = fmt.Sprintf("insert into %s values %s", tableName, generateOneRow(colInfos))
			rowCntOneSql++
			continue
		}

		insertSQL = fmt.Sprintf("%s, %s", insertSQL, generateOneRow(colInfos))
		rowCntOneSql++

		if rowCntOneSql >= batchSize {
			_, err := db.Exec(insertSQL)
			if err != nil {
				panic(fmt.Sprintf("%v", err))
			}

			insertSQL = ""
		}

		// TODO print log
	}

	if len(insertSQL) > 0 {
		_, err := db.Exec(insertSQL)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}

	}
}

func runInsertValues() {

}
