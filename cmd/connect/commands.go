package main

import (
	"codeberg.org/ale-cci/connect/pkg"
	"codeberg.org/ale-cci/connect/pkg/terminal"
	"strings"
)

type Command struct {
	Help string
	Run  func(args []string, t *terminal.Terminal, config *pkg.Config) error
}

var commands map[string]Command

func init() {
	commands = map[string]Command{
		"\\config": {
			Run:  execConfig,
			Help: "Edit runtime configuration",
		},
	}
}

func IsCommand(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return s[0] == '\\'
}

func RunCommand(cmd string, t *terminal.Terminal, config *pkg.Config) error {
	return nil
}
