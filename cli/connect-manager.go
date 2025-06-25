package main

import "os"
import "log/slog"

func help() {
	slog.Error("usage: connect-manager import <filename>")
}

func main() {
	if len(os.Args) < 3 {
		help()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "import":
		filename := os.Args[2]
		slog.Info("Importing file", "filename", filename)
	}
}
