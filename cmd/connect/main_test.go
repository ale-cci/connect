package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestWriteTableSingleLine(t *testing.T) {
	result := &ResultSet{
		Headers: []string{"ID", "Name"},
		Rows: [][]string{
			{"1", "Alice"},
			{"2", "Bob"},
		},
	}
	var buf bytes.Buffer
	writeTable(&buf, result)

	expected := " +----+-------+\n | ID | Name  |\n +----+-------+\n | 1  | Alice |\n | 2  | Bob   |\n +----+-------+\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, buf.String())
	}
}

func TestWriteTableMultiLine(t *testing.T) {
	result := &ResultSet{
		Headers: []string{"ID", "Description"},
		Rows: [][]string{
			{"1", "Line 1\nLine 2"},
			{"2", "Single line"},
		},
	}
	var buf bytes.Buffer
	writeTable(&buf, result)

	expected := " +----+-------------+\n | ID | Description |\n +----+-------------+\n | 1  | Line 1      |\n |    | Line 2      |\n | 2  | Single line |\n +----+-------------+\n"
	if buf.String() != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, buf.String())
	}
}

func TestExtractTableName(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"select * from users", "users"},
		{"SELECT a, b FROM `my_table`", "my_table"},
		{"SELECT * FROM schema.table WHERE id = 1", "schema.table"},
		{"select * from \"double_quoted\"", "double_quoted"},
		{"SELECT * from   spacing", "spacing"},
		{"INSERT INTO foo VALUES (1)", "dumped_table"},
	}

	for _, tt := range tests {
		got := extractTableName(tt.query)
		if got != tt.expected {
			t.Errorf("extractTableName(%q) = %q; expected %q", tt.query, got, tt.expected)
		}
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice", "Alice"},
		{"O'Connor", "O''Connor"},
		{"C:\\path", "C:\\\\path"},
		{"O'Connor\\Bob", "O''Connor\\\\Bob"},
	}

	for _, tt := range tests {
		got := escapeSQLString(tt.input)
		if got != tt.expected {
			t.Errorf("escapeSQLString(%q) = %q; expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatValue(t *testing.T) {
	now := time.Date(2026, 7, 2, 15, 30, 22, 0, time.UTC)
	tests := []struct {
		input    any
		expected string
	}{
		{nil, "NULL"},
		{"hello", "'hello'"},
		{[]byte("hello"), "'hello'"},
		{123, "123"},
		{int64(456), "456"},
		{123.45, "123.45"},
		{true, "1"},
		{false, "0"},
		{now, "'2026-07-02 15:30:22'"},
	}

	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.expected {
			t.Errorf("formatValue(%v) = %q; expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestSchemaElideAutoIncrement(t *testing.T) {
	autoIncrementRe := regexp.MustCompile(`(?i)\s*\bAUTO_INCREMENT\s*=\s*\d+`)
	tests := []struct {
		input    string
		expected string
	}{
		{
			"CREATE TABLE `users` (\n  `id` int NOT NULL AUTO_INCREMENT,\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB AUTO_INCREMENT=42 DEFAULT CHARSET=utf8mb4;",
			"CREATE TABLE `users` (\n  `id` int NOT NULL AUTO_INCREMENT,\n  PRIMARY KEY (`id`)\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;",
		},
		{
			"ENGINE=InnoDB AUTO_INCREMENT = 100 DEFAULT CHARSET=utf8mb4",
			"ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
		{
			"ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
			"ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
	}

	for _, tt := range tests {
		got := autoIncrementRe.ReplaceAllString(tt.input, "")
		if got != tt.expected {
			t.Errorf("expected:\n%q\ngot:\n%q", tt.expected, got)
		}
	}
}

func TestSchemaFilenameCleanup(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{"utenti", "utenti"},
		{"u%", "uall"},
		{"%", "all"},
		{"db.users*", "db_users_"},
	}

	for _, tt := range tests {
		cleanPattern := strings.ReplaceAll(tt.pattern, "%", "all")
		reClean := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
		got := reClean.ReplaceAllString(cleanPattern, "_")
		if got != tt.expected {
			t.Errorf("cleanPattern(%q) = %q; expected %q", tt.pattern, got, tt.expected)
		}
	}
}

func TestBuildSchemaQuery(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{
			"%",
			"SHOW TABLES LIKE '%'",
		},
		{
			"utenti",
			"SHOW TABLES LIKE 'utenti'",
		},
		{
			"o'connor",
			"SHOW TABLES LIKE 'o''connor'",
		},
	}

	for _, tt := range tests {
		got := buildSchemaQuery(tt.pattern)
		if got != tt.expected {
			t.Errorf("buildSchemaQuery(%q) = %q; expected %q", tt.pattern, got, tt.expected)
		}
	}
}

func TestExtractDDL(t *testing.T) {
	tests := []struct {
		name     string
		cols     []string
		vals     []any
		expected string
		err      bool
	}{
		{
			name:     "MySQL Standard (2 columns)",
			cols:     []string{"Table", "Create Table"},
			vals:     []any{"users", "CREATE TABLE `users` ..."},
			expected: "CREATE TABLE `users` ...",
			err:      false,
		},
		{
			name:     "1 Column Variant",
			cols:     []string{"Create Table"},
			vals:     []any{"CREATE TABLE `users` ..."},
			expected: "CREATE TABLE `users` ...",
			err:      false,
		},
		{
			name:     "MySQL Standard with []byte",
			cols:     []string{"Table", "Create Table"},
			vals:     []any{"users", []byte("CREATE TABLE `users` ...")},
			expected: "CREATE TABLE `users` ...",
			err:      false,
		},
		{
			name:     "Invalid columns length",
			cols:     []string{},
			vals:     []any{},
			expected: "",
			err:      true,
		},
	}

	for _, tt := range tests {
		got, err := extractDDL(tt.cols, tt.vals)
		if (err != nil) != tt.err {
			t.Errorf("%s: extractDDL() error = %v, expected err = %v", tt.name, err, tt.err)
			continue
		}
		if !tt.err && got != tt.expected {
			t.Errorf("%s: extractDDL() = %q, expected %q", tt.name, got, tt.expected)
		}
	}
}

func TestElideDatabasePrefix(t *testing.T) {
	tests := []struct {
		name      string
		createSQL string
		tableName string
		expected  string
	}{
		{
			name:      "Backticked Prefix",
			createSQL: "CREATE TABLE `mydb`.`utenti` (\n  `id` int\n)",
			tableName: "utenti",
			expected:  "CREATE TABLE `utenti` (\n  `id` int\n)",
		},
		{
			name:      "Double Quoted Prefix",
			createSQL: "CREATE TABLE \"mydb\".\"utenti\" (\n  \"id\" int\n)",
			tableName: "utenti",
			expected:  "CREATE TABLE \"utenti\" (\n  \"id\" int\n)",
		},
		{
			name:      "Unquoted Prefix",
			createSQL: "CREATE TABLE mydb.utenti (\n  id int\n)",
			tableName: "utenti",
			expected:  "CREATE TABLE utenti (\n  id int\n)",
		},
		{
			name:      "No Prefix",
			createSQL: "CREATE TABLE `utenti` (\n  `id` int\n)",
			tableName: "utenti",
			expected:  "CREATE TABLE `utenti` (\n  `id` int\n)",
		},
	}

	for _, tt := range tests {
		got := elideDatabasePrefix(tt.createSQL, tt.tableName)
		if got != tt.expected {
			t.Errorf("%s: elideDatabasePrefix() = %q, expected %q", tt.name, got, tt.expected)
		}
	}
}

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single statement with semicolon",
			input:    "select 1;",
			expected: []string{"select 1;"},
		},
		{
			name:     "Multiple statements",
			input:    "select 1; select 2;",
			expected: []string{"select 1;", "select 2;"},
		},
		{
			name:     "Single quote with semicolon",
			input:    "select 'hello; world'; select 2;",
			expected: []string{"select 'hello; world';", "select 2;"},
		},
		{
			name:     "Double quote with semicolon",
			input:    `select "hello; world"; select 3;`,
			expected: []string{`select "hello; world";`, "select 3;"},
		},
		{
			name:     "Escaped quote",
			input:    "select 'hello\\'; world'; select 4;",
			expected: []string{"select 'hello\\'; world';", "select 4;"},
		},
		{
			name:     "No trailing semicolon",
			input:    "select 1; select 2",
			expected: []string{"select 1;", "select 2"},
		},
		{
			name:     "Newlines and whitespace",
			input:    "  \n select 1;\n\nselect 2;  \n",
			expected: []string{"select 1;", "select 2;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitStatements(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d statements, got %d: %q", len(tt.expected), len(got), got)
			}
			for i, stmt := range got {
				if stmt != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], stmt)
				}
			}
		})
	}
}
