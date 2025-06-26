package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"connect/pkg"
)

func main() {
	config, err := pkg.LoadConfig(pkg.ConfigPath("config.yaml"))
	if err != nil {
		slog.Error("Failed to read config file", "err", err)
		return
	}

	if len(os.Args) <= 1 {
		slog.Error("alias obbligatorio per collegarsi a db")
		os.Exit(1)
	}
	alias := os.Args[1]

	info, ok := config.Databases[alias]
	if !ok {
		slog.Error("Alias not found in config file", "alias", alias)
		return
	}
	slog.Info("Starting connection to", "alias", alias)

	if info.Tunnel != "" {
		slog.Info("Starting tunnel", "host", info.Tunnel, "port", info.Port)
		agent, err := pkg.AuthAgent()
		if err != nil {
			slog.Error("unable to connect to ssh agent", "err", err)
			return
		}

		randomPort := 1234
		pkg.TunnelInfo{
			RemoteAddr: fmt.Sprintf("%s:%d", info.Host, info.Port),
			LocalAddr:  fmt.Sprintf("127.0.0.1:%d", randomPort),
			Agent:      agent,
		}.Start()

		info.Host = "127.0.0.1"
		info.Port = randomPort
	}

	r := bufio.NewReader(os.Stdin)

    userAlias, ok := config.Credentials[info.UserAlias]
    if !ok {
        slog.Error("alias not configured", "alias", info.UserAlias)
        os.Exit(1)
    }

	db, err := sql.Open("mysql", pkg.Connection{
		Username: userAlias.Username,
		Password: userAlias.Password,
		Host:     info.Host,
		Port:     info.Port,
		Database: info.Database,
	}.Connstring())

	if err != nil {
		slog.Error("Impossibile stabilire connessione a database", "err", err)
		return
	}
	defer db.Close()

    err = db.Ping()
    if err != nil {
        slog.Error(err.Error())
        return
    }
	for {
		fmt.Printf("> ")
		cmd, err := parseCmd(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("An error has occurred:", "err", err)
		}
		slog.Debug("executing command", "cmd", cmd)

		start := time.Now()
		result, err := runQuery(db, cmd)
		elapsed := time.Since(start)

		if err != nil {
			slog.Error("Error while running query:", "err", err)
		} else if result != nil {
			display(result)
		}

		slog.Info("Execution completed", "elapsed", elapsed)
	}
}

func display(result *ResultSet) {
	colSize := []int{}
	for _, header := range result.Headers {
		colSize = append(colSize, len(header))
	}
	for _, row := range result.Rows {
		for idx, value := range row {
			colSize[idx] = max(colSize[idx], len(value))
		}
	}

	printSep := func() {
		fmt.Printf(" +")
		for _, size := range colSize {
			fmt.Print(strings.Repeat("-", size+2), "+")
		}
		fmt.Print("\n")
	}

	printSep()
	fmts := []string{}
	for _, size := range colSize {
		fmts = append(fmts, fmt.Sprintf(" | %%-%ds", size))
	}

	for i, hdr := range result.Headers {
		fmt.Printf(fmts[i], hdr)
	}
	fmt.Print(" |\n")
	printSep()

	for _, row := range result.Rows {
		for i, item := range row {
			fmt.Printf(fmts[i], item)
		}
		fmt.Print(" |\n")
	}
	printSep()
}

func parseCmd(r *bufio.Reader) (string, error) {
	cmd := []byte{}

	escape := false
	for {
		chr, err := r.ReadByte()
		if err != nil {
			return "", err
		}

		if chr == '"' {
			escape = !escape
		} else if chr == ';' && !escape {
			break
		}

		cmd = append(cmd, chr)
	}

	return strings.TrimSpace(string(cmd)), nil
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

type ResultSet struct {
	Headers []string
	Rows    [][]string
}
