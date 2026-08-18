package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const defaultAddress = "10.2.12.124"
const defaultPort = "8001"
const defaultDBName = "test"
const defaultUser = "root"

type dbConfig struct {
	address string
	port    string
	user    string
	dbName  string
	params  []string
}

func applyDBCLIOverrides(config *dbConfig, overrides dbCLIOverrides) {
	if overrides.address.set {
		config.address = overrides.address.value
	}
	if overrides.port.set {
		config.port = overrides.port.value
	}
	if overrides.user.set {
		config.user = overrides.user.value
	}
	if overrides.dbName.set {
		config.dbName = overrides.dbName.value
	}
}

func buildDSN(config dbConfig) string {
	// Commonly, the password is always empty in my development environment
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", config.user, "", config.address, config.port, config.dbName)
	if len(config.params) > 0 {
		dsn += "?" + strings.Join(config.params, "&")
	}
	return dsn
}

func getDB(config dbConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", buildDSN(config))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func query(db *sql.DB, q string, splitter string) ([]string, error) {
	rowValues := make([]string, 0)
	rows, err := db.Query(q)
	if err != nil {
		return rowValues, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return rowValues, err
	}
	values := make([]any, len(cols))
	results := make([]sql.NullString, len(cols))
	for i := range values {
		values[i] = &results[i]
	}
	for rows.Next() {
		if err := rows.Scan(values...); err != nil {
			return rowValues, err
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
		return rowValues, err
	}
	return rowValues, nil
}

// generateRandomString returns a UTF-8 string whose rune length is in
// [minLen, maxLen]. The character set contains common ASCII characters and
// characters from several Unicode scripts, so generated values can exercise
// multi-byte character handling.
func generateRandomString(minLen, maxLen int) string {
	if minLen < 0 {
		minLen = 0
	}
	if maxLen < 0 {
		maxLen = 0
	}
	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}

	length := minLen
	if maxLen > minLen {
		length += rand.Intn(maxLen - minLen + 1)
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 " +
		"!@#$%^&*()-_=+[]{};:,.?/" +
		"中文测试你好世界" +
		"あいうえおアイウエオ" +
		"ПриветМир" +
		"مرحباالعالم" +
		"😀🚀🌟🍀"
	runes := []rune(charset)
	result := make([]rune, length)
	for i := range result {
		result[i] = runes[rand.Intn(len(runes))]
	}
	return string(result)
}

func generateRandomFloat() string {
	return fmt.Sprintf("%d.%02d", rand.Int63n(1_000_000_000), rand.Intn(100))
}

// generateRandomTime returns a MySQL TIME literal in the range 00:00:00 to
// 23:59:59.
func generateRandomTime() string {
	return fmt.Sprintf("'%02d:%02d:%02d'", rand.Intn(24), rand.Intn(60), rand.Intn(60))
}

// generateRandomDate returns a valid MySQL DATE literal between 2000-01-01
// and 2099-12-31.
func generateRandomDate() string {
	const daysInOneHundredYears = 36525
	start := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	return fmt.Sprintf("'%s'", start.AddDate(0, 0, rand.Intn(daysInOneHundredYears)).Format("2006-01-02"))
}

// generateRandomDatetime returns a valid MySQL DATETIME literal between
// 2000-01-01 00:00:00 and 2099-12-31 23:59:59.
func generateRandomDatetime() string {
	const secondsInOneHundredYears = int64(36525 * 24 * 60 * 60)
	start := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	return fmt.Sprintf("'%s'", start.Add(time.Duration(rand.Int63n(secondsInOneHundredYears))*time.Second).Format("2006-01-02 15:04:05"))
}
