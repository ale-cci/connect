package main

import (
	"context"
	"database/sql"
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
