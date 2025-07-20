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
	"strconv"
	"strings"
	"time"
	"unicode"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"

	"codeberg.org/ale-cci/connect/pkg"
	"codeberg.org/ale-cci/connect/pkg/terminal"
)

var version string = "?"

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

	if alias == "--completions" {
		aliases := []string{}
		for name := range config.Databases {
			aliases = append(aliases, name)
		}
		fmt.Printf("%s", strings.Join(aliases, " "))
		os.Exit(0)
	}

	if alias == "-v" || alias == "--version" {
		fmt.Printf(
			"connect version %s\n",
			version,
		)
		os.Exit(0)
	}

	info, ok := config.Databases[alias]
	if !ok {
		slog.Error("Alias not found in config file", "alias", alias)
		return
	}
	slog.Info("Starting connection to", "host", info.Host, "db", info.Database)

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

	userAlias, ok := config.Credentials[info.UserAlias]
	if !ok {
		slog.Error("alias not configured", "alias", info.UserAlias)
		os.Exit(1)
	}

	db, err := sql.Open(info.Driver, pkg.Connection{
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

	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer terminal.Restore(int(os.Stdin.Fd()), oldState)

	t := terminal.Terminal{
		Input:  *bufio.NewReader(os.Stdin),
		Output: os.Stdout,
		Prompt: "> ",
	}

	histfilePath := pkg.ConfigPath("history.txt")
	fd, err := os.Open(histfilePath)
	if err == nil {
		t.History.Load(fd)
		fd.Close()
	} else {
		slog.Debug("Failed to read history", "err", err)
	}

	defer func() {
		fd, err := os.Create(histfilePath)
		if err == nil {
			t.History.Save(fd)
			fd.Close()
		}
	}()

	for {
		cmd, err := t.ReadCmd()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Error("An error has occurred:", "err", err)
		}
		slog.Debug("executing command", "cmd", cmd)

		start := time.Now()

		var result *ResultSet

		if cmd == "" {
			fmt.Println("^C")
			continue
		}
		if runCmd(cmd, &t) {
			result = nil
		} else {
			result, err = runQuery(db, cmd)
		}

		elapsed := time.Since(start)

		if err != nil {
			slog.Error("Error while running query:", "err", err)
		} else {
			if result != nil {
				if len(result.Headers) > 0 {
					display(result)
				}
				slog.Info("Execution completed", "elapsed", elapsed, "rows", len(result.Rows))
			} else {
				slog.Info("Execution completed", "elapsed", elapsed)
			}
		}
	}
}

func runCmd(cmd string, t *terminal.Terminal) bool {
	tokens := tokenize(strings.TrimSpace(strings.TrimSuffix(cmd, ";")))

	// commands := [][]string{
	// 	{"\\show", {"config", "rowlimit", "tabsize", "histlen"}},
	// 	{"\\set", {"rowlimit", "tabsize", "histlen"}, int},
	// 	{"\\save", {"rowlimit", "tabsize", "histlen", "all"}},
	// 	{"\\export", string, string},
	// }

	var err error
	if tokens[0][0] == '\\' {
		switch tokens[0] {
		case "\\config":
			err = execConfig(tokens[1:], t)
		default:
			err = fmt.Errorf("command not found")
		}

		if err != nil {
			slog.Error("Command execution failed", "err", err)
		}
		return true
	}
	return false
}

func tokenize(cmd string) []string {
	tokens := []string{}

	var token []rune
	for _, chr := range cmd {
		if unicode.IsSpace(chr) {
			if len(token) > 0 {
				tokens = append(tokens, string(token))
				token = []rune{}
			}
		} else {
			token = append(token, chr)
		}
	}

	if len(token) > 0 {
		tokens = append(tokens, string(token))
	}
	return tokens
}

type Accessor struct {
	get func() string
	set func(value string) error
}

func IntValueAccessor(addr *int) Accessor {
	return Accessor{
		get: func() string {
			return fmt.Sprintf("%d", *addr)
		},
		set: func(value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			*addr = parsed
			return nil
		},
	}
}

func execConfig(tokens []string, t *terminal.Terminal) error {
	if len(tokens) == 0 {
		return fmt.Errorf("\\config {show,set,save,reset}")
	}

	configs := map[string]struct {
		get func() string
		set func(string) error
	}{
		"histsize": IntValueAccessor(&t.History.Limit),
		"autolimit": IntValueAccessor(&t.RowLimit),
		"tabsize": IntValueAccessor(&t.TabSize),
	}


	switch tokens[0] {
	case "show":
		name := tokens[1]
		c, ok := configs[name]
		if !ok {
			return fmt.Errorf("config %s does not exist")
		}

		slog.Info("show", name, c.get())

	case "set":
		name := tokens[1]
		c, ok := configs[name]
		if !ok {
			return fmt.Errorf("config %s does not exist")
		}

		return c.set(tokens[2])

	case "reset":
		slog.Info("configuration loaded from file")

	case "save":
		slog.Info("configuration saved")

	default:
		return fmt.Errorf("\\config {show,set,save}")
	}
	return nil
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
