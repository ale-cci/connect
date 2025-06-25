package pkg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
	"os/user"
	"path"
)

type Connection struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
}

func (c Connection) Connstring() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", c.Username, c.Password, c.Host, c.Port, c.Database)
}

type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ConnectionInfo struct {
	Engine    string `yaml:"engine"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	UserAlias string `yaml:"alias"`
	Database  string `yaml:"database"`
	Tunnel    string `yaml:"tunnel"`
}

type Config struct {
	Credentials map[string]User           `yaml:"credentials"`
	Databases   map[string]ConnectionInfo `yaml:"databases"`
}

func LoadConfig(filepath string) (cnf Config, err error) {
	yamlFile, err := os.ReadFile(filepath)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(yamlFile, &cnf)
	return
}

func ConfigPath(filename string) string {
	usr, _ := user.Current()
	dir := usr.HomeDir
	return path.Join(dir, ".config/connect", filename)
}
