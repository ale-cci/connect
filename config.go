package main

import (
	"database/sql"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	"os/user"
	"path"
)

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

type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ConnectionInfo struct {
	Engine    string `yaml:"engine"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	UserAlias string `yaml:"alias"`
	Database  string `yaml:"database"`
	Tunnel    string `yaml:"tunnel"`
}

type Config struct {
	Credentials map[string]User           `yaml:"credentials"`
	Databases   map[string]ConnectionInfo `yaml:"databases"`
}

func LoadConfig(filepath string) (cnf Config, err error) {
	yamlFile, err := os.ReadFile(filepath)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(yamlFile, &cnf)
	return
}

func ConfigPath(filename string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	return path.Join(dir, ".config/connect", filename)
}
