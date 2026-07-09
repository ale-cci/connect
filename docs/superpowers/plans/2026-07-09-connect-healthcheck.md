# Client Configuration Healthcheck Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a concurrent `--healthcheck` flag in the `connect` CLI tool to test all client database configurations in parallel with a strict 5-second timeout, dynamic SSH tunnel ports, and a beautiful, barebones final summary.

**Architecture:** 
- Parse the `--healthcheck` command-line argument early in `cmd/connect/main.go`.
- Implement healthcheck execution and parallel worker coordination in a new file `cmd/connect/healthcheck.go`.
- Use a worker pool of up to 5 concurrent goroutines.
- Dynamically allocate local TCP ports for SSH tunnels to prevent collisions and race conditions.
- Collect results in memory, sort alphabetically by alias, and print a clean final summary to the terminal.

**Tech Stack:** Go (Standard Library: `database/sql`, `net`, `sync`, `context`, `sort`, `fmt`).

## Global Constraints

- No emojis in console output.
- Print results clearly and minimalistically with `[OK]` and `[ERROR]` indicators.
- Set a strict 5-second timeout per database connection/ping.
- Maximum of 5 concurrent workers checking database connections in parallel.
- No changes to Git state (do not stage or commit) unless explicitly requested.

---

### Task 1: Healthcheck Structure & Single-Alias Validation Logic (TDD)

**Files:**
- Create: `cmd/connect/healthcheck.go`
- Create: `cmd/connect/healthcheck_test.go`

**Interfaces:**
- Produces: `type HealthcheckResult struct`
- Produces: `func checkDatabase(ctx context.Context, alias string, info pkg.ConnectionInfo, config pkg.Config) HealthcheckResult`

- [ ] **Step 1: Write failing tests for single-alias configuration check**
  Create `cmd/connect/healthcheck_test.go` verifying that:
  - Missing credentials alias returns an error: `"alias not configured"`.
  - An invalid database driver (e.g. `"invalid-driver"`) returns an error during verification.

```go
package main

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/ale-cci/connect/pkg"
)

func TestCheckDatabaseMissingCredentials(t *testing.T) {
	config := pkg.Config{
		Credentials: map[string]pkg.User{}, // empty credentials
		Databases: map[string]pkg.ConnectionInfo{
			"test-db": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "missing-user",
				Database:  "mydb",
				Driver:    "mysql",
			},
		},
	}

	res := checkDatabase(context.Background(), "test-db", config.Databases["test-db"], config)
	if res.Success {
		t.Error("expected failure for missing credentials")
	}
	if !strings.Contains(res.Err.Error(), "alias not configured") {
		t.Errorf("expected 'alias not configured' error, got %v", res.Err)
	}
}

func TestCheckDatabaseInvalidDriver(t *testing.T) {
	config := pkg.Config{
		Credentials: map[string]pkg.User{
			"my-user": {Username: "root", Password: "pwd"},
		},
		Databases: map[string]pkg.ConnectionInfo{
			"test-db": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "my-user",
				Database:  "mydb",
				Driver:    "unknown-driver",
			},
		},
	}

	res := checkDatabase(context.Background(), "test-db", config.Databases["test-db"], config)
	if res.Success {
		t.Error("expected failure for invalid driver")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "sql: unknown driver") {
		t.Errorf("expected sql unknown driver error, got %v", res.Err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**
  Run the test using `go test`:
  `go test ./cmd/connect/...`
  Expected output: Compilation failure because `checkDatabase` and `HealthcheckResult` are not defined.

- [ ] **Step 3: Implement minimal structure in `cmd/connect/healthcheck.go`**
  Write `cmd/connect/healthcheck.go` with minimal code to pass the tests.

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"codeberg.org/ale-cci/connect/pkg"
)

type HealthcheckResult struct {
	Alias    string
	Host     string
	Database string
	Success  bool
	Err      error
}

func checkDatabase(ctx context.Context, alias string, info pkg.ConnectionInfo, config pkg.Config) HealthcheckResult {
	res := HealthcheckResult{
		Alias:    alias,
		Host:     info.Host,
		Database: info.Database,
	}

	userAlias, ok := config.Credentials[info.UserAlias]
	if !ok {
		res.Err = fmt.Errorf("alias not configured: %s", info.UserAlias)
		return res
	}

	db, err := sql.Open(info.Driver, pkg.Connection{
		Username: userAlias.Username,
		Password: userAlias.Password,
		Host:     info.Host,
		Port:     info.Port,
		Database: info.Database,
	}.Connstring())
	if err != nil {
		res.Err = err
		return res
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		res.Err = err
		return res
	}

	res.Success = true
	return res
}
```

- [ ] **Step 4: Run tests to verify they pass**
  Run: `go test ./cmd/connect/...`
  Expected: PASS

---

### Task 2: Implement Dynamic Port Tunneling Support

**Files:**
- Modify: `cmd/connect/healthcheck.go`

**Interfaces:**
- Consumes: `pkg.TunnelInfo`, `pkg.AuthAgent()`
- Produces: Integrated tunnel handling inside `checkDatabase` that starts SSH tunnel on dynamic port `:0` and stops listener when check completes.

- [ ] **Step 1: Update `checkDatabase` to handle SSH tunnels**
  Modify `checkDatabase` in `cmd/connect/healthcheck.go` to handle `info.Tunnel` dynamically:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"

	"codeberg.org/ale-cci/connect/pkg"
)

// checkDatabase executes the health check for a single configuration, supporting SSH tunnels
func checkDatabase(ctx context.Context, alias string, info pkg.ConnectionInfo, config pkg.Config) HealthcheckResult {
	res := HealthcheckResult{
		Alias:    alias,
		Host:     info.Host,
		Database: info.Database,
	}

	userAlias, ok := config.Credentials[info.UserAlias]
	if !ok {
		res.Err = fmt.Errorf("alias not configured: %s", info.UserAlias)
		return res
	}

	// Dynamic tunnel setup
	if info.Tunnel != "" {
		agent, err := pkg.AuthAgent()
		if err != nil {
			res.Err = fmt.Errorf("unable to connect to ssh agent: %w", err)
			return res
		}

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			res.Err = fmt.Errorf("failed to start local listener: %w", err)
			return res
		}
		defer listener.Close()

		localPort := listener.Addr().(*net.TCPAddr).Port
		values := strings.SplitN(info.Tunnel, "@", 2)
		if len(values) < 2 {
			res.Err = fmt.Errorf("invalid tunnel configuration: %s", info.Tunnel)
			return res
		}

		go pkg.TunnelInfo{
			User:       values[0],
			SshAddr:    fmt.Sprintf("%s:22", values[1]),
			RemoteAddr: fmt.Sprintf("%s:%d", info.Host, info.Port),
			Agent:      agent,
		}.Start(listener)

		info.Host = "127.0.0.1"
		info.Port = localPort
	}

	db, err := sql.Open(info.Driver, pkg.Connection{
		Username: userAlias.Username,
		Password: userAlias.Password,
		Host:     info.Host,
		Port:     info.Port,
		Database: info.Database,
	}.Connstring())
	if err != nil {
		res.Err = err
		return res
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		res.Err = err
		return res
	}

	res.Success = true
	return res
}
```

- [ ] **Step 2: Run unit tests to verify nothing broke**
  Run: `go test ./cmd/connect/...`
  Expected: PASS

---

### Task 3: Implement Parallel Worker Pool, Alphabetic Sort, and Report Generator

**Files:**
- Modify: `cmd/connect/healthcheck.go`
- Create: Add test verifying multi-database processing in `cmd/connect/healthcheck_test.go`

**Interfaces:**
- Produces: `func RunHealthcheck(config pkg.Config, out io.Writer) error`

- [ ] **Step 1: Implement the concurrent RunHealthcheck function**
  Add `RunHealthcheck` to `cmd/connect/healthcheck.go` using a worker pool of size 5:

```go
import (
	"io"
	"sort"
	"sync"
	"time"
)

func RunHealthcheck(config pkg.Config, out io.Writer) error {
	type task struct {
		alias string
		info  pkg.ConnectionInfo
	}

	var aliases []string
	for alias := range config.Databases {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	fmt.Fprintf(out, "Checking %d databases...\n\n", len(aliases))

	tasksChan := make(chan task, len(aliases))
	resultsChan := make(chan HealthcheckResult, len(aliases))

	// Populate tasks
	for _, alias := range aliases {
		tasksChan <- task{alias: alias, info: config.Databases[alias]}
	}
	close(tasksChan)

	// Launch worker pool (max 5 concurrent workers)
	numWorkers := 5
	if len(aliases) < numWorkers {
		numWorkers = len(aliases)
	}

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasksChan {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				res := checkDatabase(ctx, t.alias, t.info, config)
				cancel()
				resultsChan <- res
			}
		}()
	}

	wg.Wait()
	close(resultsChan)

	// Collect and sort results alphabetically by alias
	var results []HealthcheckResult
	for res := range resultsChan {
		results = append(results, res)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Alias < results[j].Alias
	})

	passed := 0
	failed := 0

	for _, res := range results {
		if res.Success {
			fmt.Fprintf(out, "[OK]    %-20s (Host: %s, Database: %s)\n", res.Alias, res.Host, res.Database)
			passed++
		} else {
			fmt.Fprintf(out, "[ERROR] %-20s (Host: %s, Database: %s) - %v\n", res.Alias, res.Host, res.Database, res.Err)
			failed++
		}
	}

	fmt.Fprintf(out, "\nSummary: %d passed, %d failed, 0 skipped.\n", passed, failed)
	
	if failed > 0 {
		return fmt.Errorf("%d healthchecks failed", failed)
	}
	return nil
}
```

- [ ] **Step 2: Add test for RunHealthcheck**
  Add a test to `cmd/connect/healthcheck_test.go` to verify sorting and summary report generation on a mock/sample configuration:

```go
import (
	"bytes"
)

func TestRunHealthcheckReporting(t *testing.T) {
	// A config with invalid database definitions so they fail immediately but report predictably
	config := pkg.Config{
		Credentials: map[string]pkg.User{
			"my-user": {Username: "root", Password: "pwd"},
		},
		Databases: map[string]pkg.ConnectionInfo{
			"db-b": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "my-user",
				Database:  "mydb",
				Driver:    "unknown-driver",
			},
			"db-a": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "my-user",
				Database:  "mydb",
				Driver:    "unknown-driver",
			},
		},
	}

	var buf bytes.Buffer
	err := RunHealthcheck(config, &buf)

	if err == nil {
		t.Error("expected error due to failures")
	}

	output := buf.String()

	// Verify order is alphabetical
	idxA := strings.Index(output, "db-a")
	idxB := strings.Index(output, "db-b")

	if idxA == -1 || idxB == -1 {
		t.Error("missing databases in report output")
	}
	if idxA > idxB {
		t.Error("databases are not sorted alphabetically in report")
	}

	if !strings.Contains(output, "Summary: 0 passed, 2 failed, 0 skipped.") {
		t.Errorf("unexpected summary text: %s", output)
	}
}
```

- [ ] **Step 3: Run the test suite**
  Run: `go test -v ./cmd/connect/...`
  Expected: PASS

---

### Task 4: Hook Flag in `main.go` and Manual Verification

**Files:**
- Modify: `cmd/connect/main.go`

**Interfaces:**
- Consumes: `RunHealthcheck(config, os.Stdout)`

- [ ] **Step 1: Check and parse `--healthcheck` flag in `main.go`**
  Modify `cmd/connect/main.go` around line 55 (where arguments are checked) to parse `--healthcheck`:

```go
	if alias == "--healthcheck" {
		err := RunHealthcheck(config, os.Stdout)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}
```

- [ ] **Step 2: Run the full Go test suite to ensure zero regressions**
  Run: `go test ./...`
  Expected: PASS

- [ ] **Step 3: Build the application to verify compilation**
  Run: `go build -o connect ./cmd/connect`
  Expected: Successful compilation, binary `connect` produced.
