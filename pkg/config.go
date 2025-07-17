package pkg

import (
	"bytes"
	"net/url"
	"os"
	"strconv"

	"os/user"
	"path"

	"gopkg.in/yaml.v2"
)

type Connection struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string
}

// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func (c Connection) Connstring() string {
	var buf bytes.Buffer

	if len(c.Username) > 0 {
		buf.WriteString(c.Username)
		buf.WriteByte(':')
		buf.WriteString(c.Password)
		buf.WriteByte('@')
	}
	if c.Port == 0 {
		buf.WriteString("unix")
	} else {
		buf.WriteString("tcp")
	}

	buf.WriteByte('(')
	buf.WriteString(c.Host)
	if c.Port != 0 {
		buf.WriteByte(':')
		buf.WriteString(strconv.Itoa(c.Port))
	}
	buf.WriteByte(')')

	buf.WriteByte('/')
	buf.WriteString(url.PathEscape(c.Database))
	return buf.String()
}

type User struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type ConnectionInfo struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	UserAlias string `yaml:"alias"`
	Database  string `yaml:"database"`
	Tunnel    string `yaml:"tunnel"`
	Driver    string `yaml:"driver"`
}

type ConfigOptions struct {
	AutoLimit int `yaml:"autolimit"`
	HistSize  int `yaml:"histsize"`
	TabSize   int `yaml:"tabsize"`
}

type Config struct {
	Credentials map[string]User           `yaml:"credentials"`
	Databases   map[string]ConnectionInfo `yaml:"databases"`
	Options     ConfigOptions             `yaml:"options"`
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
