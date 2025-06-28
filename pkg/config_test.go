package pkg_test

import "testing"
import "codeberg.org/ale-cci/connect/pkg"

func TestDSNFormatting(t *testing.T) {
	table := []struct {
		conn   pkg.Connection
		expect string
	}{
		{
			conn: pkg.Connection{
				Host: "127.0.0.1",
				Port: 3306,
			},
			expect: "tcp(127.0.0.1:3306)/",
		},
		{
			conn: pkg.Connection{
				Username: "username",
				Password: "password",
				Host:     "host.docker.internal",
				Port:     3306,
				Database: "dbname",
			},
			expect: "username:password@tcp(host.docker.internal:3306)/dbname",
		},
		{
			conn: pkg.Connection{
				Host:     "/var/run/mysql.sock",
				Username: "admin",
				Password: "admin",
				Database: "mysql",
			},
			expect: "admin:admin@unix(/var/run/mysql.sock)/mysql",
		},
	}

	for _, tt := range table {
		got := tt.conn.Connstring()

		if tt.expect != got {
			t.Errorf("expect %v, got %v", tt.expect, got)
		}
	}
}
