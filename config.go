package main

import (
	"database/sql"
	"fmt"
)

type ConnectionInfo struct {
	Engine     string
	Host       string
	Port       string
	User       string
	Database   string
	TunnelHost string
}

type Connection struct {
	Username string
	Password string
	Host     string
	Port     string
	Database string
}

func (c Connection) Connstring() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", c.Username, c.Password, c.Host, c.Port, c.Database)
}

type ResultSet struct {
	Headers []string
	Rows    [][]string
}

func runQuery(db *sql.DB, cmd string) (results *ResultSet, err error) {
	rows, err := db.Query(cmd)
	if err != nil {
		return
	}

	cols, _ := rows.Columns()

	results = &ResultSet{}

	for _, colname := range cols {
		results.Headers = append(results.Headers, colname)
	}

	currentRow := make([]any, len(cols))
	for idx := range cols {
		var i []byte
		currentRow[idx] = &i
	}

	for rows.Next() {
		err := rows.Scan(currentRow...)

		if err != nil {
			return results, err
		}

		parsed := []string{}
		for _, ptr := range currentRow {
			parsed = append(parsed, string(*ptr.(*[]byte)))
		}
		results.Rows = append(results.Rows, parsed)
	}

	return
}
