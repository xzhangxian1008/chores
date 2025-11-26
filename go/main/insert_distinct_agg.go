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
	stringStringVals = make([][]string, 0, 10)
	stringStringVals = append(stringStringVals, []string{"fefsg4", "32Fwg4f"})
	stringStringVals = append(stringStringVals, []string{"oRn$4g", "f*7g4"})
	stringStringVals = append(stringStringVals, []string{"0HF83#F", "#$f898grG"})
	stringStringVals = append(stringStringVals, []string{"J$g94G#$", "f(#F#38f)"})
	stringStringVals = append(stringStringVals, []string{"(8feF)", "hfeiIFh4"})
	stringStringVals = append(stringStringVals, []string{"OifI8fe4", "98Fei3Fe"})
	stringStringVals = append(stringStringVals, []string{"fefsG4", "32FWg4f"})
	stringStringVals = append(stringStringVals, []string{"greRG", "65Ggf"})
	stringStringVals = append(stringStringVals, []string{"4GF4grG", "rj64"})
	stringStringVals = append(stringStringVals, []string{"shtR45", "4fgG4gr"})
}

func fillStringIntVals() {
	stringIntVals = make([]map[string]int, 0, 10)
	stringIntVals = append(stringIntVals, map[string]int{"fefsg4": 14})
	stringIntVals = append(stringIntVals, map[string]int{"oRn$4g": 45})
	stringIntVals = append(stringIntVals, map[string]int{"0HF83#F": 643})
	stringIntVals = append(stringIntVals, map[string]int{"J$g94G#$": 21})
	stringIntVals = append(stringIntVals, map[string]int{"(8feF)": 641})
	stringIntVals = append(stringIntVals, map[string]int{"OifI8fe4": 41})
	stringIntVals = append(stringIntVals, map[string]int{"fefsG4": 471})
	stringIntVals = append(stringIntVals, map[string]int{"greRG": 61})
	stringIntVals = append(stringIntVals, map[string]int{"4GF4grG": 13})
	stringIntVals = append(stringIntVals, map[string]int{"shtR45": 71})
}

// t1 int: create table t1 (col0 int, col1 int);
// t2 float: create table t2 (col0 double, col1 int);
// t3 decimal: create table t3 (col0 decimal(20, 2), col1 int);
// t4 string ci collation: create table t4 (col0 varchar(30), col1 int)collate=utf8_general_ci;
// t5 string cs collation: create table t5 (col0 varchar(30), col1 int)collate=utf8_bin;
// t6 string string: create table t6 (col0 varchar(30), col1 varchar(30), col2 int);
// t7 string int: create table t7 (col0 varchar(30), col1 int, col2 int);

func generateGroupByVal(randomRange int) int {
	return rand.Intn(randomRange)
}

func generateT1Row() string {
	return fmt.Sprintf("(%d, %d)", intVals[rand.Intn(len(intVals))], generateGroupByVal(nvdNum))
}

func generateT2OrT3Row() string {
	return fmt.Sprintf("(%f, %d)", floatOrDecimalVals[rand.Intn(len(floatOrDecimalVals))], generateGroupByVal(nvdNum))
}

func generateT4OrT5Row() string {
	return fmt.Sprintf("('%s', %d)", stringVals[rand.Intn(len(stringVals))], generateGroupByVal(nvdNum))
}

func generateT6Row() string {
	idx := rand.Intn(len(stringStringVals))
	return fmt.Sprintf("('%s', '%s', %d)", stringStringVals[idx][0], stringStringVals[idx][1], generateGroupByVal(totalDistinctAggRowNum/10))
}

func generateT7Row() string {
	tmp := stringIntVals[rand.Intn(len(stringIntVals))]
	if len(tmp) != 1 {
		panic("len(tmp) != 1")
	}
	for k, v := range tmp {
		return fmt.Sprintf("('%s', %d, %d)", k, v, generateGroupByVal(totalDistinctAggRowNum/10))
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
	case "t6":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT6Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT6Row())
		}
	case "t7":
		insertSql = fmt.Sprintf("%s %s", insertSql, generateT7Row())
		for range rowNum {
			insertSql = fmt.Sprintf("%s, %s", insertSql, generateT7Row())
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
	nvdNum = totalDistinctAggRowNum / 10
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
	tableNames := []string{"t1", "t2", "t3", "t4", "t5"}
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
