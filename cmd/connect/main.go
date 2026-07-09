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
	"regexp"
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

func loadconfig(t *terminal.Terminal, c *pkg.Config) {
	t.History.Size = c.Options.HistSize
	t.TabSize = c.Options.TabSize
	t.RowLimit = c.Options.AutoLimit
}

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

	fdStdin := int(os.Stdin.Fd())
	if terminal.IsTerminal(fdStdin) {
		oldState, err := terminal.MakeRaw(fdStdin)
		if err != nil {
			panic(err)
		}
		defer terminal.Restore(fdStdin, oldState)

		t := terminal.Terminal{
			Input:  *bufio.NewReader(os.Stdin),
			Output: os.Stdout,
			Prompt: "> ",
			Fd:     fdStdin,
			State:  oldState,
		}
		loadconfig(&t, &config)

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
				continue
			}
			slog.Debug("executing command", "cmd", cmd)

			start := time.Now()

			var result *ResultSet

			if cmd == "" {
				fmt.Println("^C")
				continue
			}
			if IsCommand(cmd) {
				result = nil
				err = RunCommand(cmd, db, &t, &config)
			} else {
				if t.RowLimit > 0 {
					newcmd, replaced := AddLimit(cmd, t.RowLimit)
					cmd = newcmd
					if replaced {
						slog.Info("Autolimit added", "limit", t.RowLimit)
					}
				}

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
	} else {
		// Non-interactive mode (piped or redirected input)
		inputBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			slog.Error("Failed to read stdin", "err", err)
			os.Exit(1)
		}

		t := terminal.Terminal{
			Output: os.Stdout,
			Prompt: "> ",
			Fd:     fdStdin,
		}
		loadconfig(&t, &config)

		statements := splitStatements(string(inputBytes))
		for _, cmd := range statements {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}

			slog.Debug("executing non-interactive command", "cmd", cmd)
			start := time.Now()

			var result *ResultSet

			if IsCommand(cmd) {
				result = nil
				err = RunCommand(cmd, db, &t, &config)
			} else {
				result, err = runQuery(db, cmd)
			}

			elapsed := time.Since(start)

			if err != nil {
				slog.Error("Error while running query:", "err", err)
				os.Exit(1)
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
}

func AddLimit(cmd string, limit int) (string, bool) {
	cleanedCmd := strings.TrimRightFunc(strings.ToLower(strings.TrimSpace(cmd)), func(r rune) bool {
		return unicode.IsDigit(r) || unicode.IsSpace(r) || r == ';'
	})

	isSelect := strings.HasPrefix(cleanedCmd, "select")
	if isSelect && !strings.HasSuffix(cleanedCmd, "limit") {
		cmd = fmt.Sprintf("%s limit %d;", strings.TrimSuffix(strings.TrimSpace(cmd), ";"), limit)
		return cmd, true
	}
	return cmd, false
}

func display(result *ResultSet) {
	writeTable(os.Stdout, result)
}

func writeTable(w io.Writer, result *ResultSet) {
	colSize := []int{}
	for _, header := range result.Headers {
		colSize = append(colSize, len(header))
	}

	// Split all row cells into lines and find max width per column
	splitRows := make([][][]string, len(result.Rows))
	for rIdx, row := range result.Rows {
		splitRows[rIdx] = make([][]string, len(row))
		for cIdx, value := range row {
			lines := strings.Split(value, "\n")
			splitRows[rIdx][cIdx] = lines
			for _, line := range lines {
				colSize[cIdx] = max(colSize[cIdx], len(line))
			}
		}
	}

	printSep := func() {
		fmt.Fprint(w, " +")
		for _, size := range colSize {
			fmt.Fprint(w, strings.Repeat("-", size+2), "+")
		}
		fmt.Fprint(w, "\n")
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
		fmt.Fprintf(w, fmts[i], hdr)
	}
	fmt.Fprint(w, " |\n")
	printSep()

	for _, splitRow := range splitRows {
		// Determine the height of this row
		rowHeight := 0
		for _, cellLines := range splitRow {
			rowHeight = max(rowHeight, len(cellLines))
		}

		// Print each sub-line for the row
		for lineIdx := 0; lineIdx < rowHeight; lineIdx++ {
			for colIdx := range colSize {
				var lineText string
				if colIdx < len(splitRow) {
					cellLines := splitRow[colIdx]
					if lineIdx < len(cellLines) {
						lineText = cellLines[lineIdx]
					}
				}
				fmt.Fprintf(w, fmts[colIdx], lineText)
			}
			fmt.Fprint(w, " |\n")
		}
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

type Command struct {
	Help string
	Run  func(args []string, db *sql.DB, t *terminal.Terminal, commands map[string]Command, config *pkg.Config) error
}

var commands map[string]Command

func init() {
	commands = map[string]Command{
		"\\config": {
			Run:  execConfig,
			Help: "Edit runtime configuration",
		},
		"\\help": {
			Run:  execHelp,
			Help: "Run this command",
		},
		"\\dump": {
			Run:  execDump,
			Help: "Esegue una query e salva i risultati come istruzioni INSERT in un file dump-nome_tabella-data-ora.sql",
		},
		"\\schema": {
			Run:  execSchema,
			Help: "Salva lo schema (DDL) delle tabelle corrispondenti al pattern in un file schema-pattern-data-ora.sql",
		},
	}
}

func IsCommand(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return s[0] == '\\'
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

func RunCommand(cmd string, db *sql.DB, t *terminal.Terminal, config *pkg.Config) error {
	tokens := tokenize(strings.TrimSpace(strings.TrimSuffix(cmd, ";")))

	commandName := tokens[0]

	command, ok := commands[commandName]
	if !ok {
		slog.Error("Command not found", "value", commandName)
	} else {
		err := command.Run(tokens[1:], db, t, commands, config)
		if err != nil {
			slog.Error("Command execution failed", "err", err)
		}
	}

	return nil
}

func execHelp(args []string, db *sql.DB, t *terminal.Terminal, commands map[string]Command, config *pkg.Config) error {
	fmt.Println("Comandi disponibili:")

	for name, cmd := range commands {
		fmt.Printf(" %8s: %s\n", name, cmd.Help)
	}
	fmt.Println("")
	return nil
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

func execConfig(tokens []string, db *sql.DB, t *terminal.Terminal, _ map[string]Command, config *pkg.Config) error {
	if len(tokens) == 0 {
		return fmt.Errorf("\\config {show,set}")
	}

	configs := map[string]struct {
		get func() string
		set func(string) error
	}{
		"histsize":  IntValueAccessor(&t.History.Size),
		"autolimit": IntValueAccessor(&t.RowLimit),
		"tabsize":   IntValueAccessor(&t.TabSize),
	}

	switch tokens[0] {
	case "get":
		result := ResultSet{
			Headers: []string{"Name", "Value"},
		}

		if len(tokens) > 1 {
			name := tokens[1]
			c, ok := configs[name]
			if !ok {
				return fmt.Errorf("config %s does not exist", name)
			}

			result.Rows = [][]string{
				{name, c.get()},
			}
		} else {
			for name, attr := range configs {
				result.Rows = append(result.Rows, []string{
					name, attr.get(),
				})
			}
		}

		display(&result)

	case "set":
		if len(tokens) > 1 {
			name := tokens[1]
			c, ok := configs[name]
			if !ok {
				return fmt.Errorf("config %s does not exist", name)
			}

			return c.set(tokens[2])
		}
		return fmt.Errorf("config set <name> <value>")

	case "reset":
		loadconfig(t, config)

	default:
		return fmt.Errorf("\\config {get,set}")
	}
	return nil
}

func execDump(tokens []string, db *sql.DB, t *terminal.Terminal, _ map[string]Command, config *pkg.Config) error {
	if len(tokens) == 0 {
		return fmt.Errorf("sintassi: \\dump <select_query>")
	}

	query := strings.Join(tokens, " ")
	tableName := extractTableName(query)

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("errore durante l'esecuzione della query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("errore nel recupero delle colonne: %w", err)
	}

	filename := "dump-" + tableName + "-" + time.Now().Format("20060102-1504") + ".sql"
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("errore nella creazione del file %s: %w", filename, err)
	}
	defer file.Close()

	// Write header comments
	fmt.Fprintf(file, "-- Dump generato il %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "-- Query: %s\n", query)
	fmt.Fprintf(file, "-- Tabella: %s\n\n", tableName)

	// Wrap columns in backticks to prevent issues with reserved SQL words
	colNames := make([]string, len(columns))
	for i, col := range columns {
		colNames[i] = "`" + col + "`"
	}
	colsStr := strings.Join(colNames, ", ")

	// Prepare values scanning
	values := make([]any, len(columns))
	valuePtrs := make([]any, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("errore durante lo scan dei risultati: %w", err)
		}

		vals := make([]string, len(columns))
		for i, val := range values {
			vals[i] = formatValue(val)
		}
		valsStr := strings.Join(vals, ", ")

		// Format as INSERT INTO `table` (`col1`, `col2`) VALUES ('val1', 'val2');
		fmt.Fprintf(file, "INSERT INTO `%s` (%s) VALUES (%s);\n", tableName, colsStr, valsStr)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("errore durante l'iterazione delle righe: %w", err)
	}

	fmt.Printf("Dump completato: %d righe salvate in %s\n", rowCount, filename)
	return nil
}

func extractTableName(query string) string {
	re := regexp.MustCompile(`(?i)\bfrom\s+([a-zA-Z0-9_\x60\x22.-]+)`)
	matches := re.FindStringSubmatch(query)
	if len(matches) < 2 {
		return "dumped_table"
	}
	tableName := matches[1]
	tableName = strings.ReplaceAll(tableName, "`", "")
	tableName = strings.ReplaceAll(tableName, "\"", "")
	return tableName
}

func formatValue(val any) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case string:
		return "'" + escapeSQLString(v) + "'"
	case []byte:
		return "'" + escapeSQLString(string(v)) + "'"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case time.Time:
		return "'" + v.Format("2006-01-02 15:04:05") + "'"
	default:
		return "'" + escapeSQLString(fmt.Sprintf("%v", v)) + "'"
	}
}

func escapeSQLString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "''")
	return s
}

func buildSchemaQuery(pattern string) string {
	escapedPattern := escapeSQLString(pattern)
	return fmt.Sprintf("SHOW TABLES LIKE '%s'", escapedPattern)
}

func execSchema(tokens []string, db *sql.DB, t *terminal.Terminal, _ map[string]Command, config *pkg.Config) error {
	if len(tokens) == 0 {
		return fmt.Errorf("sintassi: \\schema <pattern>")
	}

	pattern := strings.Join(tokens, " ")

	// Find tables matching the pattern using dynamically built query to be immune to prepared statement bugs or limitations
	query := buildSchemaQuery(pattern)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("errore nel recupero delle tabelle con pattern %s: %w", pattern, err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("errore durante lo scan dei nomi delle tabelle: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("errore durante l'iterazione delle tabelle: %w", err)
	}

	if len(tables) == 0 {
		fmt.Printf("Nessuna tabella trovata corrispondente al pattern: %s\n", pattern)
		return nil
	}

	// Create safe filename
	cleanPattern := strings.ReplaceAll(pattern, "%", "all")
	reClean := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	cleanPattern = reClean.ReplaceAllString(cleanPattern, "_")

	filename := "schema-" + cleanPattern + "-" + time.Now().Format("20060102-1504") + ".sql"
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("errore nella creazione del file %s: %w", filename, err)
	}
	defer file.Close()

	// Write header comments
	fmt.Fprintf(file, "-- Schema generato il %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "-- Pattern: %s\n", pattern)
	fmt.Fprintf(file, "-- Tabelle trovate: %s\n\n", strings.Join(tables, ", "))

	autoIncrementRe := regexp.MustCompile(`(?i)\s*\bAUTO_INCREMENT\s*=\s*\d+`)

	schemaCount := 0
	for _, table := range tables {
		rows, err := db.Query(fmt.Sprintf("SHOW CREATE TABLE `%s`", table))
		if err != nil {
			return fmt.Errorf("errore nel recupero ddl per la tabella %s: %w", table, err)
		}

		if !rows.Next() {
			rows.Close()
			return fmt.Errorf("nessun risultato ddl per la tabella %s", table)
		}

		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return fmt.Errorf("errore nel recupero delle colonne ddl per la tabella %s: %w", table, err)
		}

		vals := make([]any, len(cols))
		valPtrs := make([]any, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			rows.Close()
			return fmt.Errorf("errore durante lo scan ddl per la tabella %s: %w", table, err)
		}
		rows.Close()

		createSQL, err := extractDDL(cols, vals)
		if err != nil {
			return fmt.Errorf("errore nell'estrazione ddl per la tabella %s: %w", table, err)
		}

		// Elide auto-increment if present
		createSQL = autoIncrementRe.ReplaceAllString(createSQL, "")

		// Elide database prefix if present
		createSQL = elideDatabasePrefix(createSQL, table)

		fmt.Fprintf(file, "-- Struttura della tabella: %s\n", table)
		fmt.Fprintf(file, "%s;\n\n", createSQL)
		schemaCount++
	}

	fmt.Printf("Schema completato: %d definizioni salvate in %s\n", schemaCount, filename)
	return nil
}

func elideDatabasePrefix(createSQL, tableName string) string {
	// Pattern for backticked prefix: `mydb`.`table_name` -> `table_name`
	reBacktick := regexp.MustCompile(fmt.Sprintf("(?i)`[a-zA-Z0-9_-]+`\\.`%s`", regexp.QuoteMeta(tableName)))
	createSQL = reBacktick.ReplaceAllString(createSQL, "`"+tableName+"`")

	// Pattern for double-quoted prefix: "mydb"."table_name" -> "table_name"
	reQuote := regexp.MustCompile(fmt.Sprintf("(?i)\"[a-zA-Z0-9_-]+\"\\.\"%s\"", regexp.QuoteMeta(tableName)))
	createSQL = reQuote.ReplaceAllString(createSQL, "\""+tableName+"\"")

	// Pattern for unquoted prefix: mydb.table_name -> table_name
	reUnquoted := regexp.MustCompile(fmt.Sprintf("(?i)\\b[a-zA-Z0-9_-]+\\.%s\\b", regexp.QuoteMeta(tableName)))
	createSQL = reUnquoted.ReplaceAllString(createSQL, tableName)

	return createSQL
}

func extractDDL(cols []string, vals []any) (string, error) {
	if len(cols) == 0 || len(vals) < len(cols) {
		return "", fmt.Errorf("colonne o valori non validi")
	}

	var ddl any
	if len(cols) >= 2 {
		ddl = vals[1]
	} else {
		ddl = vals[0]
	}

	if ddl == nil {
		return "", fmt.Errorf("ddl nullo")
	}

	switch v := ddl.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func splitStatements(input string) []string {
	var statements []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escapeNext := false

	for i := 0; i < len(input); i++ {
		r := rune(input[i])
		if escapeNext {
			current.WriteRune(r)
			escapeNext = false
			continue
		}

		if r == '\\' {
			current.WriteRune(r)
			escapeNext = true
			continue
		}

		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
		} else if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
		}

		current.WriteRune(r)

		if r == ';' && !inSingleQuote && !inDoubleQuote {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
