package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
)

func Try(xs ...any) any {
	if len(xs) == 0 {
		return nil
	}
	if err, ok := xs[len(xs)-1].(error); ok && err != nil {
		panic(err)
	}
	return xs[0]
}

const table1Name = "t1"
const table2Name = "t2"
const table3Name = "t3"
const table4Name = "t4"
const table5Name = "t5"
const table6Name = "t6"
const table7Name = "t7"
const table8Name = "t8"
const table9Name = "t9"
const table10Name = "t10"
const table11Name = "t11"
const table12Name = "t12"

const varcharLen = 30
const generatedNumberCeiling = 100000000

var createTable1SQL = fmt.Sprintf("create table %s (c0 varchar(%d), c1 int, index idx1(c0(10))) charset=binary collate=binary;", table1Name, varcharLen)
var createTable2SQL = fmt.Sprintf("create table %s (c0 varchar(%d), c1 int, index idx1(c0(10))) charset=utf8mb4 collate=utf8mb4_bin;", table2Name, varcharLen)
var createTable3SQL = fmt.Sprintf("create table %s (c0 text, c1 int, index idx1(c0(10))) charset=utf8mb4 collate=utf8mb4_general_ci;", table3Name)
var createTable4SQL = fmt.Sprintf("create table %s (c0 varchar(%d), c1 int, index idx1(c0(10))) charset=gb18030 collate=gb18030_bin;", table4Name, varcharLen)
var createTable5SQL = fmt.Sprintf("create table %s (c0 text, c1 int, index idx1(c0(10))) charset=gb18030 collate=gb18030_chinese_ci;", table5Name)
var createTable6SQL = fmt.Sprintf("create table %s (c0 blob, c1 int, index idx1(c0(10)));", table6Name)
var createTable7SQL = fmt.Sprintf("create table %s (c0 varchar(30), c1 int, index idx1(c0));", table7Name)
var createTable8SQL = fmt.Sprintf("create table %s(c0 varchar(%d), c1 text, index idx1(c0, c1(10)));", table8Name, varcharLen)
var createTable9SQL = fmt.Sprintf("create table %s(c0 varchar(%d), c1 varchar(%d), index idx1(c0, c1));", table9Name, varcharLen, varcharLen)
var createTable10SQL = fmt.Sprintf("create table %s(c0 text, c1 varchar(%d), c2 text, index idx1(c0(10), c1));", table10Name, varcharLen)
var createTable11SQL = fmt.Sprintf("create table %s(c0 blob, c1 text, c2 varchar(%d), index idx1(c0(10), c2));", table11Name, varcharLen)
var createTable12SQL = fmt.Sprintf("create table %s(c0 varchar(%d), c1 varchar(%d), c2 varchar(%d), index idx1(c0, c2(10)));", table12Name, varcharLen, varcharLen, varcharLen)

var binTableName = []string{table8Name}

// var binTableName = []string{table1Name, table2Name, table4Name, table6Name, table7Name, table8Name, table9Name, table10Name, table11Name, table12Name}
// var ciTableNames = []string{table3Name, table5Name}
var ciTableNames = []string{}

type testCase struct {
	fullDataSql string
	testSql     string
	limit       int
	offset      int
}

var queriesForCS = []testCase{}
var queriesForCI = []testCase{}

var t1RowCount int
var t2RowCount int
var t3RowCount int
var t4RowCount int
var t5RowCount int
var t6RowCount int
var t7RowCount int
var t8RowCount int
var t9RowCount int
var t10RowCount int
var t11RowCount int
var t12RowCount int

var chars = []string{
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l",
	"m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x",
	"y", "z", "A", "B", "C", "D", "E", "F", "G", "H", "I", "J",
	"K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V",
	"W", "X", "Y", "Z",
	"显", "不", "起", "的", "本", "活", "担", "生", "基", "开",
	"经", "甚", "敦", "济", "院", "字", "是", "学", "数", "伦",
	"࿙", "⩻", "ඇ", "Ʃ", "희", "౽", "இ", "ŭ", "ᢴ", "ᨓ", "ⵎ",
	"🎉", "🍹", "🎁", "🍺", "🔥", "🌟", "✨", "😊", "🍩",
}

var charsLen = len(chars)

func generateTestQueries() {
	csTableNames := []string{table1Name, table2Name, table4Name, table6Name, table7Name}

	for _, tbName := range csTableNames {
		limit := rand.Intn(300000)
		offset := rand.Intn(300000)

		// Pattern: select c0, c1 from xx order by c0 limit xx offset xx
		queriesForCS = append(queriesForCS, testCase{
			fullDataSql: fmt.Sprintf("select c0, c1 from %s", tbName),
			testSql:     fmt.Sprintf("select c0, c1 from %s order by c0 limit %d offset %d", tbName, limit, offset),
			limit:       limit,
			offset:      offset,
		})

		limit = rand.Intn(100000)
		offset = rand.Intn(100000)

		// Pattern: select c0, c1 from xx where length(c0) > xx order by c0, c1 limit xx offset xx
		filterParam := rand.Intn(generatedNumberCeiling)
		queriesForCS = append(queriesForCS, testCase{
			fullDataSql: fmt.Sprintf("select c0, c1 from %s where length(c0) > %d", tbName, filterParam),
			testSql:     fmt.Sprintf("select c0, c1 from %s where length(c0) > %d order by c0, c1 limit %d offset %d", tbName, filterParam, limit, offset),
			limit:       limit,
			offset:      offset,
		})
	}

	ciTableNames := []string{table3Name, table5Name}
	for _, tbName := range ciTableNames {
		limit := rand.Intn(300000)
		offset := rand.Intn(300000)
		queriesForCI = append(queriesForCI, testCase{
			fullDataSql: fmt.Sprintf("select c0 from %s", tbName),
			testSql:     fmt.Sprintf("select c0 from %s order by c0 limit %d offset %d", tbName, limit, offset),
			limit:       limit,
			offset:      offset,
		})
	}
}

func getRandBinString(strLen int) string {
	bytes := make([]byte, 0, strLen)
	for len(bytes) < strLen {
		appendedChar := []byte(chars[rand.Intn(charsLen)])
		if len(bytes)+len(appendedChar) <= strLen {
			bytes = append(bytes, appendedChar...)
		}
	}
	return string(bytes)
}

func getRandString(charCount int) string {
	bytes := make([]byte, 0, charCount*4)
	for range charCount {
		bytes = append(bytes, []byte(chars[rand.Intn(charsLen)])...)
	}
	return string(bytes)
}

// This prefix is appended to the string so as to test ci and cs collation
var prefixStr = []string{"pre", "PRE", "pRe", "PrE", "PRe"}
var prefixStrLen int = len(prefixStr)

const maxRepeatCount = 5

var generateBinString = func(sameCount *int, cachedString *string) string {
	if *sameCount == 0 {
		*sameCount = rand.Intn(maxRepeatCount) + 1
		*cachedString = getRandBinString(rand.Intn(varcharLen) + 1)
	}
	(*sameCount)--
	return *cachedString
}

var generateCIString = func(sameCount *int, cachedString *string) string {
	if *sameCount == 0 {
		*sameCount = rand.Intn(maxRepeatCount) + 1
		*cachedString = getRandString(rand.Intn(varcharLen-3) + 1) // -3 is for prefixStr
	}
	(*sameCount)--
	return fmt.Sprintf("%s%s", prefixStr[rand.Intn(prefixStrLen)], *cachedString)
}

var generateCSString = func(sameCount *int, cachedString *string) string {
	if *sameCount == 0 {
		*sameCount = rand.Intn(maxRepeatCount) + 1
		*cachedString = getRandString(rand.Intn(varcharLen) + 1)
	}
	(*sameCount)--
	return *cachedString
}

func generateNullAndEmptyStrTwoCols(tbName string, db *sql.DB) int {
	// Insert some null values for index column
	insertSQL := fmt.Sprintf("insert into %s values (null, %d)", tbName, rand.Intn(100000000))
	insertNullValRowNum := rand.Intn(500)
	for i := 1; i < insertNullValRowNum; i++ {
		if i%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values (null, %d)", tbName, rand.Intn(100000000))
			continue
		}
		insertSQL = fmt.Sprintf("%s, (null, %d)", insertSQL, rand.Intn(100000000))
	}

	if insertNullValRowNum > 0 {
		// Insert remaining data
		Try(db.Exec(insertSQL))
	}

	// Insert some empty strings for index column
	insertSQL = fmt.Sprintf("insert into %s values ('', %d)", tbName, rand.Intn(100000000))
	insertEmptyStringRowNum := rand.Intn(500)
	for i := 1; i < insertEmptyStringRowNum; i++ {
		if i%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values ('', %d)", tbName, rand.Intn(100000000))
			continue
		}
		insertSQL = fmt.Sprintf("%s, ('', %d)", insertSQL, rand.Intn(100000000))
	}

	if insertEmptyStringRowNum > 0 {
		// Insert remaining data
		Try(db.Exec(insertSQL))
	}
	return insertNullValRowNum + insertEmptyStringRowNum
}

func generateDataForOneTableTwoCols(db *sql.DB, tbName string, generateString func(*int, *string) string) int {
	expectedRowNum := rand.Intn(800000) + 200000

	cachedString := ""
	sameCount := 0

	insertedRowNum := generateNullAndEmptyStrTwoCols(tbName, db)
	insertSQL := fmt.Sprintf("insert into %s values ('%s', %d)", tbName, generateString(&sameCount, &cachedString), rand.Intn(generatedNumberCeiling))
	insertedRowNum++
	for ; insertedRowNum < expectedRowNum; insertedRowNum++ {
		if insertedRowNum%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values ('%s', %d)", tbName, generateString(&sameCount, &cachedString), rand.Intn(generatedNumberCeiling))
			continue
		}
		insertSQL = fmt.Sprintf("%s, ('%s', %d)", insertSQL, generateString(&sameCount, &cachedString), rand.Intn(generatedNumberCeiling))
	}

	// Insert remaining data
	Try(db.Exec(insertSQL))
	return expectedRowNum
}

func generateNullAndEmptyStrThreeCols(tbName string, db *sql.DB) int {
	// Insert some null values for index column
	insertSQL := fmt.Sprintf("insert into %s values (null, null, null)", tbName)
	insertNullValRowNum := rand.Intn(50)
	for i := 1; i < insertNullValRowNum; i++ {
		if i%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values (null, null, null)", tbName)
			continue
		}
		insertSQL = fmt.Sprintf("%s, (null, null, null)", insertSQL)
	}

	if insertNullValRowNum > 0 {
		// Insert remaining data
		Try(db.Exec(insertSQL))
	}

	// Insert some empty strings for index column
	insertSQL = fmt.Sprintf("insert into %s values ('', '', '')", tbName)
	insertEmptyStringRowNum := rand.Intn(50)
	for i := 1; i < insertEmptyStringRowNum; i++ {
		if i%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values ('', '', '')", tbName)
			continue
		}
		insertSQL = fmt.Sprintf("%s, ('', '', '')", insertSQL)
	}

	if insertEmptyStringRowNum > 0 {
		// Insert remaining data
		Try(db.Exec(insertSQL))
	}
	return insertNullValRowNum + insertEmptyStringRowNum
}

func generateDataForOneTableThreeCols(db *sql.DB, tbName string, generateString func(*int, *string) string, candidateStr []string) int {
	expectedRowNum := rand.Intn(80000) + 20000

	cachedString := ""
	sameCount := 0

	insertedRowNum := generateNullAndEmptyStrThreeCols(tbName, db)
	insertSQL := fmt.Sprintf("insert into %s values ('%s', '%s', '%s')", tbName, generateString(&sameCount, &cachedString), candidateStr[rand.Intn(len(candidateStr))], candidateStr[rand.Intn(len(candidateStr))])
	insertedRowNum++
	for ; insertedRowNum < expectedRowNum; insertedRowNum++ {
		if insertedRowNum%1000 == 0 {
			Try(db.Exec(insertSQL))
			insertSQL = fmt.Sprintf("insert into %s values ('%s', '%s', '%s')", tbName, generateString(&sameCount, &cachedString), candidateStr[rand.Intn(len(candidateStr))], candidateStr[rand.Intn(len(candidateStr))])
			continue
		}
		insertSQL = fmt.Sprintf("%s, ('%s', '%s', '%s')", insertSQL, generateString(&sameCount, &cachedString), candidateStr[rand.Intn(len(candidateStr))], candidateStr[rand.Intn(len(candidateStr))])
	}

	// Insert remaining data
	Try(db.Exec(insertSQL))
	return expectedRowNum
}

func runWorker(tbName string, db *sql.DB) {
	candidateStr := make([]string, 0, 100)
	for range cap(candidateStr) {
		candidateStr = append(candidateStr, getRandString(rand.Intn(30)))
	}

	switch tbName {
	case table1Name:
		t1RowCount = generateDataForOneTableTwoCols(db, table1Name, generateBinString)
	case table2Name:
		t2RowCount = generateDataForOneTableTwoCols(db, table2Name, generateCSString)
	case table3Name:
		t3RowCount = generateDataForOneTableTwoCols(db, table3Name, generateCIString)
	case table4Name:
		t4RowCount = generateDataForOneTableTwoCols(db, table4Name, generateCSString)
	case table5Name:
		t5RowCount = generateDataForOneTableTwoCols(db, table5Name, generateCIString)
	case table6Name:
		t6RowCount = generateDataForOneTableTwoCols(db, table6Name, generateCIString)
	case table7Name:
		t7RowCount = generateDataForOneTableTwoCols(db, table7Name, generateCIString)
	case table8Name:
		t8RowCount = generateDataForOneTableTwoCols(db, table8Name, generateCIString)
	case table9Name:
		t9RowCount = generateDataForOneTableTwoCols(db, table9Name, generateCIString)
	case table10Name:
		t10RowCount = generateDataForOneTableThreeCols(db, table10Name, generateCIString, candidateStr)
	case table11Name:
		t11RowCount = generateDataForOneTableThreeCols(db, table11Name, generateCIString, candidateStr)
	case table12Name:
		t12RowCount = generateDataForOneTableThreeCols(db, table12Name, generateCIString, candidateStr)
	default:
		panic(fmt.Sprintf("Unknown table %s", tbName))
	}
}

func main() {
	a := make([]int, 5)
	clear(a)
	fmt.Println(len(a))

	return

	db, err := getDB()
	if err != nil {
		panic(fmt.Sprintf("Fail to get db. error: %v", err))
	}

	defer func() {
		db.Close()
	}()

	waiter := &sync.WaitGroup{}
	waiter.Add(len(binTableName) + len(ciTableNames))
	for _, tbName := range binTableName {
		go func(tbName string) {
			defer fmt.Printf("table %s done\n", tbName)
			defer waiter.Done()
			runWorker(tbName, db)
		}(tbName)
	}

	for _, tbName := range ciTableNames {
		go func(tbName string) {
			defer fmt.Printf("table %s done\n", tbName)
			defer waiter.Done()
			runWorker(tbName, db)
		}(tbName)
	}
	waiter.Wait()
}
