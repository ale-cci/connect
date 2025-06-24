package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Credential struct {
	Username string
	Password string
}

func ReadConnections(connfile string) (connections map[string]ConnectionInfo, err error) {
	f, err := os.Open(connfile)
	if err != nil {
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)

	header, err := reader.Read()
	if err != nil {
		return
	}

	connections = map[string]ConnectionInfo{}

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		conn := ConnectionInfo{
			Engine:   "mysql",
			Database: "",
		}

		alias := ""
		for i, value := range record {
			slog.Debug("record", "r", value)
			if header[i] == "Host" {
				conn.Host = value
			} else if header[i] == "User" {
				conn.User = value
			} else if header[i] == "Engine" {
				conn.Engine = value
			} else if header[i] == "Port" {
				conn.Port = value
			} else if header[i] == "Tunnel" {
				conn.TunnelHost = value
			} else if header[i] == "Alias" {
				alias = value
			} else if header[i] == "Database" {
				conn.Database = value
			}
		}
		if alias != "" {
			connections[alias] = conn
		}
	}
	return
}

func main() {
	connections, err := ReadConnections("conn.csv")
	if err != nil {
		slog.Error("Failed to read config file", "err", err)
		return
	}

	if len(os.Args) <= 1 {
		slog.Error("alias obbligatorio per collegarsi a db")
		os.Exit(1)
	}
	alias := os.Args[1]

	info, ok := connections[alias]
	if !ok {
		slog.Error("Alias not found in config file", "alias", alias)
		return
	}
	slog.Info("Starting connection to", "alias", alias)

	if info.TunnelHost != "" {
		slog.Info("Starting tunnel", "host", info.TunnelHost, "port", info.Port)
	}

	r := bufio.NewReader(os.Stdin)

	db, err := sql.Open("mysql", Connection{
		Username: "",
		Password: "",
		Host:     info.Host,
		Port:     info.Port,
		Database: info.Database,
	}.Connstring())

	if err != nil {
		slog.Error("Impossibile stabilire connessione a database", "err", err)
		return
	}
	defer db.Close()

	for {
		fmt.Printf("> ")
		cmd, err := parseCmd(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("An error has occurred:", "err", err)
		}
		slog.Info("executing command", "cmd", cmd)

		result, err := runQuery(db, cmd)


		if err != nil {
			slog.Error("Error while running query:", "err", err)
		} else if result != nil {

			for _, hdr := range result.Headers {
				fmt.Printf("%s\t", hdr)
			}
			fmt.Print("\n")

			for _, row := range result.Rows {
				for _, item := range row {
					fmt.Printf("%s\t", item)
				}
				fmt.Print("\n")
			}

		}
	}
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
