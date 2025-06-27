package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
    "unicode"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"codeberg.org/ale-cci/connect/pkg"
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
		randomPort := rand.Intn(1000) + 9000
		slog.Info("Starting tunnel", "host", info.Tunnel, "port", info.Port, "localport", randomPort)
		agent, err := pkg.AuthAgent()
		if err != nil {
			slog.Error("unable to connect to ssh agent", "err", err)
			os.Exit(1)
		}

		localAddr := fmt.Sprintf("127.0.0.1:%d", randomPort)
		listener, err := net.Listen("tcp", localAddr)
		if err != nil {
			slog.Error("failed to start local listener", "err", err)
			os.Exit(1)
		}

		values := strings.SplitN(info.Tunnel, "@", 2)
		defer listener.Close()
		go pkg.TunnelInfo{
			User:       values[0],
			SshAddr:    fmt.Sprintf("%s:22", values[1]),
			RemoteAddr: fmt.Sprintf("%s:%d", info.Host, info.Port),
			Agent:      agent,
		}.Start(listener)

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

	slog.Info("pinging the database")
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
			if len(result.Headers) > 0 {
				display(result)
			}
			slog.Info("Execution completed", "elapsed", elapsed, "rows", len(result.Rows))
		}
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
		hdr = strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return '•'
		}, hdr)
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

	var escapeChr byte = '\x00'

	for {
		chr, err := r.ReadByte()
		if err != nil {
			return "", err
		}

		if chr == '"' || chr == '\'' {
			if escapeChr == chr {
				escapeChr = '\x00'
			} else if escapeChr == '\x00' {
				escapeChr = chr
			}
		} else if chr == ';' && escapeChr == '\x00' {
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
