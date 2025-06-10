package main

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

type ConnectionInfo struct {
	Engine     string
	Host       string
	Port       int
	User       string
	Database   string
	TunnelHost string
}

type Credential struct {
	Username string
	Password string
}

func (c ConnectionInfo) mingle() string {
	return fmt.Sprintf("%s", c.Engine)
}

func ReadConnections(connfile string) (connections map[string]ConnectionInfo, err error) {
	f, err := os.Open(connfile)
	if err != nil {
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)

	header, err := reader.Read()
	if err != nil {
		return
	}

	connections = map[string]ConnectionInfo{}

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		conn := ConnectionInfo{
			Engine: "mysql",
		}

		for i, value := range record {
			slog.Debug("record", "r", value)
			if header[i] == "Host" {
				conn.Host = value
			} else if header[i] == "User" {
			}
		}

		connections[conn.mingle()] = conn
	}
	return
}

func main() {
	connections, err := ReadConnections("conn.csv")
	if err != nil {
		slog.Error("Failed to read config file", "err", err)
		return
	}

	alias := os.Args[1]
	slog.Info("Opening connection to", "alias", alias)

	info, ok := connections[alias]
	if !ok {
		slog.Error("Alias not found in config file", "alias", alias)
		return
	}

	if info.TunnelHost != "" {
		// start the tunnel
	}

	cmd := exec.Command("echo", info.Host)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}
