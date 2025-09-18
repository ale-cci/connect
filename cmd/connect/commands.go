package main

import (
	"codeberg.org/ale-cci/connect/pkg"
	"codeberg.org/ale-cci/connect/pkg/terminal"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode"
)

type Command struct {
	Help string
	Run  func(args []string, t *terminal.Terminal, commands map[string]Command, config *pkg.Config) error
}

var commands map[string]Command

func init() {
	commands = map[string]Command{
		"\\config": {
			Run:  execConfig,
			Help: "Edit runtime configuration",
		},
		"\\help": {
			Run:  execHelp,
			Help: "Run this command",
		},
		"\\schema": {
			Run:  execDump,
			Help: "salva lo schema del database su file",
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

func tokenize(cmd string) []string {
	tokens := []string{}

	var token []rune
	for _, chr := range cmd {
		if unicode.IsSpace(chr) {
			if len(token) > 0 {
				tokens = append(tokens, string(token))
				token = []rune{}
			}
		} else {
			token = append(token, chr)
		}
	}

	if len(token) > 0 {
		tokens = append(tokens, string(token))
	}
	return tokens
}

func RunCommand(cmd string, db *sql.DB, t *terminal.Terminal, config *pkg.Config) error {
	tokens := tokenize(strings.TrimSpace(strings.TrimSuffix(cmd, ";")))

	commandName := tokens[0]

	command, ok := commands[commandName]
	if !ok {
		slog.Error("Command not found", "value", commandName)
	} else {
		err := command.Run(tokens[1:], t, commands, config)
		if err != nil {
			slog.Error("Command execution failed", "err", err)
		}
	}

	return nil
}

func execHelp(args []string, t *terminal.Terminal, commands map[string]Command, config *pkg.Config) error {
	fmt.Println("Comandi disponibili:")

	for name, cmd := range commands {
		fmt.Printf(" %8s: %s\n", name, cmd.Help)
	}
	fmt.Println("")
	return nil
}

type Accessor struct {
	get func() string
	set func(value string) error
}

func IntValueAccessor(addr *int) Accessor {
	return Accessor{
		get: func() string {
			return fmt.Sprintf("%d", *addr)
		},
		set: func(value string) error {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			*addr = parsed
			return nil
		},
	}
}

func execConfig(tokens []string, t *terminal.Terminal, _ map[string]Command, config *pkg.Config) error {
	if len(tokens) == 0 {
		return fmt.Errorf("\\config {show,set}")
	}

	configs := map[string]struct {
		get func() string
		set func(string) error
	}{
		"histsize":  IntValueAccessor(&t.History.Size),
		"autolimit": IntValueAccessor(&t.RowLimit),
		"tabsize":   IntValueAccessor(&t.TabSize),
	}

	switch tokens[0] {
	case "get":
		result := ResultSet{
			Headers: []string{"Name", "Value"},
		}

		if len(tokens) > 1 {
			name := tokens[1]
			c, ok := configs[name]
			if !ok {
				return fmt.Errorf("config %s does not exist", name)
			}

			result.Rows = [][]string{
				{name, c.get()},
			}
		} else {
			for name, attr := range configs {
				result.Rows = append(result.Rows, []string{
					name, attr.get(),
				})
			}
		}

		display(&result)

	case "set":
		if len(tokens) > 1 {
			name := tokens[1]
			c, ok := configs[name]
			if !ok {
				return fmt.Errorf("config %s does not exist", name)
			}

			return c.set(tokens[2])
		}
		return fmt.Errorf("config set <name> <value>")

	case "reset":
		loadconfig(t, config)

	default:
		return fmt.Errorf("\\config {get,set}")
	}
	return nil
}

func execDump(tokens []string, t *terminal.Terminal, _ map[string]Command, config *pkg.Config) error {
	return nil
}
