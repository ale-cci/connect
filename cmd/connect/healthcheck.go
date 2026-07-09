package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"

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
