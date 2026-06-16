package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"codeberg.org/ale-cci/connect/pkg"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var version string = "?"

func main() {
	// Configure slog to output strictly to os.Stderr
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	config, err := pkg.LoadConfig(pkg.ConfigPath("config.yaml"))
	if err != nil {
		slog.Error("Failed to read config file", "err", err)
		os.Exit(1)
	}

	portOpt := 0
	alias := ""

	// Manual parsing of -port / --port to keep --completions and -v clean
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-port" || arg == "--port" {
			if i+1 < len(os.Args) {
				var err error
				portOpt, err = strconv.Atoi(os.Args[i+1])
				if err != nil {
					slog.Error("Porta non valida", "val", os.Args[i+1], "err", err)
					os.Exit(1)
				}
				i++ // skip the value
			} else {
				slog.Error("Opzione -port richiede un valore")
				os.Exit(1)
			}
		} else if strings.HasPrefix(arg, "-") && arg != "-v" && arg != "--version" && arg != "--completions" {
			slog.Error("Opzione non riconosciuta", "opt", arg)
			fmt.Fprintf(os.Stderr, "Usage: connect-mcp [-port <port>] <alias>\n")
			os.Exit(1)
		} else {
			alias = arg
		}
	}

	if alias == "" {
		slog.Error("alias obbligatorio per collegarsi a db in modalità MCP")
		fmt.Fprintf(os.Stderr, "Usage: connect-mcp [-port <port>] <alias>\n")
		os.Exit(1)
	}

	if alias == "--completions" {
		aliases := []string{}
		for name := range config.Databases {
			aliases = append(aliases, name)
		}
		fmt.Printf("%s", strings.Join(aliases, " "))
		os.Exit(0)
	}

	if alias == "-v" || alias == "--version" {
		fmt.Printf("connect-mcp version %s\n", version)
		os.Exit(0)
	}

	info, ok := config.Databases[alias]
	if !ok {
		slog.Error("Alias not found in config file", "alias", alias)
		os.Exit(1)
	}
	slog.Info("Starting MCP connection to", "host", info.Host, "db", info.Database)

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
		defer listener.Close()

		values := strings.SplitN(info.Tunnel, "@", 2)
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
		os.Exit(1)
	}
	defer db.Close()

	slog.Info("pinging the database")
	err = db.Ping()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	err = StartMcpServer(db, info.Database, portOpt)
	if err != nil {
		slog.Error("MCP server failed", "err", err)
		os.Exit(1)
	}
}

// StartMcpServer starts the MCP server using SSE transport and prints its address to stdout
func StartMcpServer(db *sql.DB, schemaName string, portOpt int) error {
	slog.Info("Initializing MCP server for database", "schema", schemaName)

	// Create a new MCP server
	s := server.NewMCPServer(
		"connect-mysql-mcp",
		"1.0.0",
	)

	// 1. Tool: list_tables
	listTablesTool := mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in the connected MySQL database"),
	)
	s.AddTool(listTablesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slog.Info("MCP call: list_tables")
		jsonStr, err := executeSQLToJSON(db, "SHOW TABLES;")
		if err != nil {
			slog.Error("Failed to list tables", "err", err)
			return mcp.NewToolResultError(fmt.Sprintf("Error listing tables: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	})

	// 2. Tool: describe_table
	describeTableTool := mcp.NewTool("describe_table",
		mcp.WithDescription("Get column definitions, types, keys, and default values for a specific table"),
		mcp.WithString("table_name",
			mcp.Required(),
			mcp.Description("The name of the table to describe"),
		),
	)
	s.AddTool(describeTableTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tableName, err := request.RequireString("table_name")
		if err != nil {
			slog.Error("Missing table_name argument", "err", err)
			return mcp.NewToolResultError("missing required table_name argument"), nil
		}

		slog.Info("MCP call: describe_table", "table", tableName)
		jsonStr, err := describeTable(db, schemaName, tableName)
		if err != nil {
			slog.Error("Failed to describe table", "table", tableName, "err", err)
			return mcp.NewToolResultError(fmt.Sprintf("Error describing table %q: %v", tableName, err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	})

	// 3. Tool: execute_query
	executeQueryTool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an arbitrary raw SQL query (e.g., SELECT, INSERT, UPDATE, etc.) against the database"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The SQL query to execute"),
		),
	)
	s.AddTool(executeQueryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			slog.Error("Missing query argument", "err", err)
			return mcp.NewToolResultError("missing required query argument"), nil
		}

		slog.Info("MCP call: execute_query", "query", query)
		jsonStr, err := executeSQLToJSON(db, query)
		if err != nil {
			slog.Error("Failed to execute query", "query", query, "err", err)
			return mcp.NewToolResultError(fmt.Sprintf("Error executing query: %v", err)), nil
		}
		return mcp.NewToolResultText(jsonStr), nil
	})

	port := portOpt
	if port == 0 {
		var err error
		port, err = findFreePort()
		if err != nil {
			slog.Error("Failed to allocate a free port", "err", err)
			return err
		}
	}

	// Create SSE Server
	sseServer := server.NewSSEServer(s, server.WithBaseURL(fmt.Sprintf("http://localhost:%d", port)))

	// Print the actual SSE server URL to stdout as requested by the user
	fmt.Printf("http://localhost:%d/sse\n", port)

	slog.Info("Starting SSE transport for connect-mysql-mcp", "port", port)
	if err := sseServer.Start(fmt.Sprintf(":%d", port)); err != nil {
		slog.Error("MCP server SSE transport failed", "err", err)
		return err
	}

	return nil
}

// findFreePort locates an available TCP port on localhost
func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// executeSQLToJSON runs a SQL query and serializes the resulting rows (or rows affected) into indented JSON
func executeSQLToJSON(db *sql.DB, query string) (string, error) {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) == 0 {
		return "[]", nil
	}

	// Simple routing: if it is a write command, run Exec; otherwise use Query
	firstWord := strings.ToLower(strings.Fields(trimmed)[0])
	isSelect := firstWord == "select" || firstWord == "show" || firstWord == "describe" || firstWord == "explain" || firstWord == "desc" || firstWord == "help"

	if !isSelect {
		res, err := db.Exec(query)
		if err != nil {
			return "", err
		}
		rowsAffected, _ := res.RowsAffected()
		lastInsertId, _ := res.LastInsertId()
		return fmt.Sprintf(`{"rows_affected": %d, "last_insert_id": %d}`, rowsAffected, lastInsertId), nil
	}

	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var results []map[string]any

	currentRow := make([]any, len(cols))
	for idx := range cols {
		var i []byte
		currentRow[idx] = &i
	}

	for rows.Next() {
		err := rows.Scan(currentRow...)
		if err != nil {
			return "", err
		}

		rowMap := make(map[string]any)
		for idx, colname := range cols {
			ptr := currentRow[idx]
			bPtr := ptr.(*[]byte)
			if bPtr == nil || *bPtr == nil {
				rowMap[colname] = nil
			} else {
				rowMap[colname] = string(*bPtr)
			}
		}
		results = append(results, rowMap)
	}

	if err = rows.Err(); err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "[]", nil
	}

	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// describeTable fetches detailed columns and types metadata securely from information_schema
func describeTable(db *sql.DB, schemaName, tableName string) (string, error) {
	rows, err := db.Query(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA 
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, schemaName, tableName)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var results []map[string]any

	currentRow := make([]any, len(cols))
	for idx := range cols {
		var i []byte
		currentRow[idx] = &i
	}

	for rows.Next() {
		err := rows.Scan(currentRow...)
		if err != nil {
			return "", err
		}

		rowMap := make(map[string]any)
		for idx, colname := range cols {
			ptr := currentRow[idx]
			bPtr := ptr.(*[]byte)
			if bPtr == nil || *bPtr == nil {
				rowMap[colname] = nil
			} else {
				rowMap[colname] = string(*bPtr)
			}
		}
		results = append(results, rowMap)
	}

	if err = rows.Err(); err != nil {
		return "", err
	}

	if len(results) == 0 {
		return fmt.Sprintf("Table %q not found in schema %q.", tableName, schemaName), nil
	}

	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
