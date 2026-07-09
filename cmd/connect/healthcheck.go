package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

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

		sshAddr := values[1]
		if !strings.Contains(sshAddr, ":") {
			sshAddr = fmt.Sprintf("%s:22", sshAddr)
		}

		go pkg.TunnelInfo{
			User:       values[0],
			SshAddr:    sshAddr,
			RemoteAddr: net.JoinHostPort(info.Host, fmt.Sprintf("%d", info.Port)),
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
