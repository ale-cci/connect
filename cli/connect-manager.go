package main

import (
	"connect/pkg"
	"encoding/csv"
	"errors"
    "io"
	"log/slog"
	"os"
	"strconv"
	"gopkg.in/yaml.v2"
)

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

        configpath := pkg.ConfigPath("config.yaml")

        config, err := pkg.LoadConfig(configpath)
        if err != nil {
            config = pkg.Config{
                Databases: map[string]pkg.ConnectionInfo{},
            }
        }

        err = readCsv(config, filename)
        if err != nil {
            slog.Error("failed to read csv file", "err", err)
        } else {

            file, err := os.Create(configpath)
            if err != nil {
                slog.Error("Failed to open file", "err", err)
                os.Exit(1)
            }

            if err = yaml.NewEncoder(file).Encode(config); err != nil {
                slog.Error("failed to writing config", "err", err)
                os.Exit(1)
            }
        }
	}
}


func readCsv(cnf pkg.Config, filepath string) (err error) {
    f, err := os.Open(filepath)
    if err != nil {
        return
    }
    defer f.Close()
    csvReader := csv.NewReader(f)
    header, err := csvReader.Read()

    if err != nil {
        return
    }

    for {
        line, err := csvReader.Read()
        if err != nil {
            if errors.Is(err, io.EOF) {
                break
            }
            return err
        }

        alias := ""
        info := pkg.ConnectionInfo{}

        for i, hdr := range header {
            value := line[i]
            if hdr == "alias" {
                alias = value
            } else if hdr == "host" {
                info.Host = value
            } else if hdr == "port" {
                info.Port, err = strconv.Atoi(value)
                if err != nil {
                    return err
                }
            } else if hdr == "database" {
                info.Database = value
            } else if hdr == "tunnel" {
                info.Tunnel = value
            } else if hdr == "user" {
                info.UserAlias = value
            } else if hdr == "driver" {
                info.Driver = value
            }
        }

        cnf.Databases[alias] = info
    }
    return
}
