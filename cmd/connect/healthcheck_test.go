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
