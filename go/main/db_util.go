package main

import (
	"database/sql"
	"fmt"
	"strings"
)

const addr = "10.2.12.124"
const port = "7001"
const user = "root"
const dbName = "test"

var AllowCommentParam = "comments=true"

func buildDSN(addr string, user string, port, db string, params ...string) string {
	// Commonly, the password is always empty in my development environment
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, "", addr, port, db)
	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}
	return dsn
}

func getDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", buildDSN(addr, user, port, dbName, AllowCommentParam))
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
