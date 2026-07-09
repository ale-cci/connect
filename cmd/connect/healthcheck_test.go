package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/ale-cci/connect/pkg"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// mockDriver is a simple sql driver that allows us to mock database connections in tests
type mockDriver struct{}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	// Parse the connection string to find the tcp address
	// E.g., username:password@tcp(host:port)/dbname
	idx1 := strings.Index(name, "tcp(")
	if idx1 != -1 {
		idx2 := strings.Index(name[idx1:], ")")
		if idx2 != -1 {
			addr := name[idx1+4 : idx1+idx2]
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, fmt.Errorf("mock driver failed to connect to %s: %w", addr, err)
			}
			conn.Close()
		}
	}
	return &mockConn{}, nil
}

type mockConn struct{}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return nil, nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	return nil, nil
}

func init() {
	sql.Register("mock-driver", &mockDriver{})
}

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

func TestCheckDatabaseWithValidTunnel(t *testing.T) {
	// Temporarily discard slog output to avoid log noise from asynchronous cleanup
	importSlog := true
	_ = importSlog
	// We can import log/slog if needed, but wait, we can just use slog.SetDefault
	// let's add "log/slog" to imports and use slog.SetDefault

	dbListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer dbListener.Close()
	dbAddr := dbListener.Addr().(*net.TCPAddr)

	go func() {
		for {
			conn, err := dbListener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// 2. Start a mock SSH agent server
	tmpDir, err := os.MkdirTemp("", "ssh-agent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "agent.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	keyring := agent.NewKeyring()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()

	// Temporarily set SSH_AUTH_SOCK env
	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	os.Setenv("SSH_AUTH_SOCK", sockPath)
	defer os.Setenv("SSH_AUTH_SOCK", origAuthSock)

	// 3. Generate a mock SSH host key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Start a mock SSH server
	sshConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	sshConfig.AddHostKey(signer)

	sshListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer sshListener.Close()
	sshAddr := sshListener.Addr().(*net.TCPAddr)

	go func() {
		for {
			conn, err := sshListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				_, chans, reqs, err := ssh.NewServerConn(c, sshConfig)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newChan := range chans {
					if newChan.ChannelType() != "direct-tcpip" {
						newChan.Reject(ssh.UnknownChannelType, "unknown channel")
						continue
					}

					var payload struct {
						DestAddr   string
						DestPort   uint32
						OriginAddr string
						OriginPort uint32
					}
					if err := ssh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
						newChan.Reject(ssh.ConnectionFailed, "bad payload")
						continue
					}

					// Connect to database port (mock database server)
					destConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", payload.DestAddr, payload.DestPort))
					if err != nil {
						newChan.Reject(ssh.ConnectionFailed, "dial failed")
						continue
					}

					ch, reqs, err := newChan.Accept()
					if err != nil {
						destConn.Close()
						continue
					}
					go ssh.DiscardRequests(reqs)

					go func() {
						defer ch.Close()
						defer destConn.Close()
						_, _ = io.Copy(ch, destConn)
					}()
					go func() {
						defer ch.Close()
						defer destConn.Close()
						_, _ = io.Copy(destConn, ch)
					}()
				}
			}(conn)
		}
	}()

	// 5. Build Config
	config := pkg.Config{
		Credentials: map[string]pkg.User{
			"tunnel-user": {Username: "dbuser", Password: "dbpassword"},
		},
		Databases: map[string]pkg.ConnectionInfo{
			"test-db": {
				Host:      "127.0.0.1",
				Port:      dbAddr.Port,
				UserAlias: "tunnel-user",
				Database:  "mydb",
				Driver:    "mock-driver",
				Tunnel:    fmt.Sprintf("sshuser@127.0.0.1:%d", sshAddr.Port),
			},
		},
	}

	// 6. Execute healthcheck
	res := checkDatabase(context.Background(), "test-db", config.Databases["test-db"], config)
	if !res.Success {
		t.Fatalf("expected success for checkDatabase with tunnel, got error: %v", res.Err)
	}
}

func TestCheckDatabaseTunnelSshAgentErr(t *testing.T) {
	// Temporarily unset SSH_AUTH_SOCK env to force agent connection error
	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AUTH_SOCK")
	defer os.Setenv("SSH_AUTH_SOCK", origAuthSock)

	config := pkg.Config{
		Credentials: map[string]pkg.User{
			"tunnel-user": {Username: "dbuser", Password: "dbpassword"},
		},
		Databases: map[string]pkg.ConnectionInfo{
			"test-db": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "tunnel-user",
				Database:  "mydb",
				Driver:    "mock-driver",
				Tunnel:    "sshuser@127.0.0.1:22",
			},
		},
	}

	res := checkDatabase(context.Background(), "test-db", config.Databases["test-db"], config)
	if res.Success {
		t.Fatal("expected failure when ssh agent is missing")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unable to connect to ssh agent") {
		t.Errorf("expected 'unable to connect to ssh agent' error, got %v", res.Err)
	}
}

func TestCheckDatabaseTunnelInvalidFormat(t *testing.T) {
	// Start a mock SSH agent server so agent connection succeeds
	tmpDir, err := os.MkdirTemp("", "ssh-agent-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sockPath := filepath.Join(tmpDir, "agent.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	keyring := agent.NewKeyring()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, conn)
		}
	}()

	origAuthSock := os.Getenv("SSH_AUTH_SOCK")
	os.Setenv("SSH_AUTH_SOCK", sockPath)
	defer os.Setenv("SSH_AUTH_SOCK", origAuthSock)

	config := pkg.Config{
		Credentials: map[string]pkg.User{
			"tunnel-user": {Username: "dbuser", Password: "dbpassword"},
		},
		Databases: map[string]pkg.ConnectionInfo{
			"test-db": {
				Host:      "127.0.0.1",
				Port:      3306,
				UserAlias: "tunnel-user",
				Database:  "mydb",
				Driver:    "mock-driver",
				Tunnel:    "invalid-tunnel-format-no-at-sign",
			},
		},
	}

	res := checkDatabase(context.Background(), "test-db", config.Databases["test-db"], config)
	if res.Success {
		t.Fatal("expected failure for invalid tunnel format")
	}
	if res.Err == nil || !strings.Contains(res.Err.Error(), "invalid tunnel configuration") {
		t.Errorf("expected 'invalid tunnel configuration' error, got %v", res.Err)
	}
}
