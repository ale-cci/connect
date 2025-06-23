package main

import "fmt"

type ConnectionInfo struct {
	Engine     string
	Host       string
	Port       string
	User       string
	Database   string
	TunnelHost string
}

type Connection struct {
	Username string
	Password string
	Host     string
	Port     string
}

func (c Connection) Connstring() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", c.Username, c.Password, c.Host, c.Port)
}
