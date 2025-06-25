package main

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Credential struct {
	Username string
	Password string
}

func ConfigPath(filename string) string {
    usr, _ := user.Current()
    dir := usr.HomeDir
    return path.Join(dir, ".config/connect", filename)
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

type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
type Credentials struct {
	Users map[string]User `yaml:"users"`
}

func ReadCredentials(filepath string) (cred Credentials, err error) {
	yamlFile, err := os.ReadFile(filepath)
	if err != nil {
		return
	}
	cred.Users = make(map[string]User)
	err = yaml.Unmarshal(yamlFile, &cred)
	return
}

func main() {
	connections, err := ReadConnections(ConfigPath("conn.csv"))
	if err != nil {
		slog.Error("Failed to read config file", "err", err)
		return
	}

	credentials, err := ReadCredentials(ConfigPath("users.yaml"))
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
		Username: credentials.Users[info.User].Username,
		Password: credentials.Users[info.User].Password,
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
			fmt.Print(strings.Repeat("-", size + 2), "+")
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
