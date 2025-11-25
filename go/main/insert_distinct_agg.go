package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
)

var totalDistinctAggRowNum int
var nvdNum int

var intVals []int
var floatOrDecimalVals []float64
var stringVals []string
var stringStringVals [][]string
var stringIntVals []map[string]int

func lowerOrUpperCharacter(str string) string {
	strBytes := make([]byte, 0, len(str))
	for _, c := range str {
		if c > 47 && c < 58 {
			strBytes = append(strBytes, byte(c))
			continue
		}

		randNum := rand.Intn(10)

		if c > 64 && c < 91 {
			if randNum < 5 {
				strBytes = append(strBytes, byte(c+32))
			} else {
				strBytes = append(strBytes, byte(c))
			}
			continue
		}

		if randNum < 5 {
			strBytes = append(strBytes, byte(c-32))
		} else {
			strBytes = append(strBytes, byte(c))
		}
	}
	return string(strBytes)
}

func generateDistAggRows(idx int) {
	switch idx {
	case 0:
		fillIntVals()
	case 1:
		fillFloatOrDecimalVals()
	case 2:
		fillStringVals()
	case 3:
		fillStringStringVals()
	case 4:
		fillStringIntVals()
	default:
		panic("Invalid idx")
	}
}

func fillIntVals() {
	intValsMap := make(map[int]struct{})
	for range nvdNum {
		for {
			val := rand.Intn(100000000) - 50000000
			_, ok := intValsMap[val]
			if !ok {
				intValsMap[val] = struct{}{}
				break
			}
		}
	}

	intVals = make([]int, 0, len(intValsMap))
	for key := range intValsMap {
		intVals = append(intVals, key)
	}
}

func fillFloatOrDecimalVals() {
	floatOrDecimalValsMap := make(map[float64]struct{})
	for range nvdNum {
		for {
			strVal := fmt.Sprintf("%.2f", rand.Float64()*10000000-5000000)
			val, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				panic(err)
			}
			_, ok := floatOrDecimalValsMap[val]
			if !ok {
				floatOrDecimalValsMap[val] = struct{}{}
				break
			}
		}
	}

	floatOrDecimalVals = make([]float64, 0, len(floatOrDecimalValsMap))
	for key := range floatOrDecimalValsMap {
		floatOrDecimalVals = append(floatOrDecimalVals, key)
	}
}

func fillStringVals() {
	stringValsMap := make(map[string]struct{})
	for range nvdNum / 2 {
		for {
			val := generateRandomString(rand.Intn(20))
			_, ok := stringValsMap[val]
			if !ok {
				stringValsMap[val] = struct{}{}
				stringValsMap[lowerOrUpperCharacter(val)] = struct{}{}
				break
			}
		}
	}

	stringVals = make([]string, 0, len(stringValsMap))
	for key := range stringValsMap {
		stringVals = append(stringVals, key)
	}
}

func fillStringStringVals() {
	stringStringValsMap := make(map[string][]string)
	for range nvdNum / 2 {
		for {
			str1 := generateRandomString(rand.Intn(20))
			str2 := generateRandomString(rand.Intn(20))
			str3 := lowerOrUpperCharacter(str2)
			val1 := fmt.Sprintf("%s%s", str1, str2)
			_, ok := stringStringValsMap[val1]
			if !ok {
				stringStringValsMap[val1] = []string{str1, str2}
				stringStringValsMap[fmt.Sprintf("%s%s", str1, str3)] = []string{str1, str3}
				break
			}
		}
	}

	stringStringVals = make([][]string, 0, len(stringStringValsMap))
	for _, val := range stringStringValsMap {
		tmp := make([]string, 0, 2)
		tmp = append(tmp, val[0])
		tmp = append(tmp, val[1])
		stringStringVals = append(stringStringVals, tmp)
	}
}

func fillStringIntVals() {
	stringIntValsMap := make(map[string]map[string]int)
	for range nvdNum / 2 {
		for {
			str1 := generateRandomString(rand.Intn(20))
			str2 := lowerOrUpperCharacter(str1)
			intVal := rand.Intn(10000000)
			val1 := fmt.Sprintf("%s%d", str1, intVal)
			_, ok := stringIntValsMap[val1]
			if !ok {
				stringIntValsMap[val1] = map[string]int{str1: intVal}
				stringIntValsMap[fmt.Sprintf("%s%d", str2, intVal)] = map[string]int{str2: intVal}
				break
			}
		}
	}

	stringIntVals = make([]map[string]int, 0, len(stringIntValsMap))
	for _, val := range stringIntValsMap {
		stringIntVals = append(stringIntVals, val)
	}
}

// t1 int: create table t1 (col0 int, col1 int);
// t2 float: create table t2 (col0 double, col1 int);
// t3 decimal: create table t3 (col0 decimal(20, 2), col1 int);
// t4 string ci collation: create table t4 (col0 varchar(30), col1 int)collate=utf8_general_ci;
// t5 string cs collation: create table t5 (col0 varchar(30), col1 int)collate=utf8_bin;
// t6 string string ci collation collation: create table t6 (col0 varchar(30), col1 varchar(30), col2 int)collate=utf8_general_ci;
// t7 string string cs collation: create table t7 (col0 varchar(30), col1 varchar(30), col2 int)collate=utf8_bin;
// t8 string int: create table t8 (col0 varchar(30), col1 int, col2 int);

func generateGroupByVal() int {
	return rand.Intn(10)
}

func generateT1Row() string {
	return fmt.Sprintf("(%d, %d)", intVals[rand.Intn(len(intVals))], generateGroupByVal())
}

func generateT2OrT3Row() string {
	return fmt.Sprintf("(%f, %d)", floatOrDecimalVals[rand.Intn(len(floatOrDecimalVals))], generateGroupByVal())
}

func generateT4OrT5Row() string {
	return fmt.Sprintf("('%s', %d)", stringVals[rand.Intn(len(stringVals))], generateGroupByVal())
}

func generateT6OrT7Row() string {
	idx := rand.Intn(len(stringStringVals))
	return fmt.Sprintf("('%s', '%s', %d)", stringStringVals[idx][0], stringStringVals[idx][1], generateGroupByVal())
}

func generateT8Row() string {
	tmp := stringIntVals[rand.Intn(len(stringIntVals))]
	if len(tmp) != 1 {
		panic("len(tmp) != 1")
	}
	for k, v := range tmp {
		return fmt.Sprintf("('%s', %d, %d)", k, v, generateGroupByVal())
	}
	panic("Shouldn't reach here")
}

func generateTableRows(tableName string, rowNum int) string {
	insertSql := fmt.Sprintf("insert into %s values", tableName)
	switch tableName {
	case "t1":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT1Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT1Row())
		}
	case "t2", "t3":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT2OrT3Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT2OrT3Row())
		}
	case "t4", "t5":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT4OrT5Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT4OrT5Row())
		}
	case "t6", "t7":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT6OrT7Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT6OrT7Row())
		}
	case "t8":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT8Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT8Row())
		}
	default:
		panic(fmt.Sprintf("Unknown table %s", tableName))
	}
	return insertSql
}

func insertTable(db *sql.DB, tableName string, rowNum int) {
	insertSql := generateTableRows(tableName, rowNum)
	_, err := db.Exec(insertSql)
	if err != nil {
		panic(fmt.Sprintf("error: %v", err))
	}
}

func runInsertDistinctAggTableWorkers() {
	fmt.Println("Start to generate data")
	totalDistinctAggRowNum = rand.Intn(200000) + 200000
	nvdNum = rand.Intn(totalDistinctAggRowNum/2) + 1
	fmt.Printf("total row num: %d, nvd: %d\n", totalDistinctAggRowNum, nvdNum)

	wg := &sync.WaitGroup{}
	wg.Add(5)
	for i := range 5 {
		go func(idx int) {
			defer func() {
				wg.Done()
				fmt.Printf("%d generate donw", idx)
			}()
			generateDistAggRows(i)
		}(i)
	}

	wg.Wait()
	fmt.Println("All data has been generated")
	tableNames := []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8"}
	wg.Add(len(tableNames))
	for _, tableName := range tableNames {
		go func(tableName string) {
			defer wg.Done()
			db, err := getDB()
			if err != nil {
				panic(fmt.Sprintf("Fail to get db. error: %v", err))
			}

			rowNumOneBatch := 1000
			for insertedRowNum := 0; insertedRowNum < totalDistinctAggRowNum; insertedRowNum += rowNumOneBatch {
				if insertedRowNum%10000 == 0 {
					fmt.Printf("%s has been inserted %d rows\n", tableName, insertedRowNum)
				}
				insertTable(db, tableName, rowNumOneBatch)
			}
		}(tableName)
	}
	wg.Wait()
	fmt.Println("All rows have been successfully inserted")
}
