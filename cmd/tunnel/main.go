package main

import (
	"flag"
	"strings"
	"net"
	"log/slog"
	"os"
	"fmt"

	"codeberg.org/ale-cci/connect/pkg"
)

func main() {
	var local string
	var remote string
	var sshAddr string // web@host.docker.internal:22

	flag.StringVar(&local, "local", "127.0.0.1:1234", "local address")
	flag.StringVar(&sshAddr, "ssh", "user@host.addr:22", "ssh address")
	flag.StringVar(&remote, "remote", "/var/lib/docker.sock", "remote addr")
	flag.Parse()

	chunks := strings.SplitN(sshAddr, "@", 2)

	localConn, err := net.Listen("tcp", local)
	if err != nil {
		slog.Error("failed to start local connection", "err", localConn)
		return
	}

	agent, err := pkg.AuthAgent()
	if err != nil {
		slog.Error("unable to connect to ssh agent", "err", err)
		os.Exit(1)
	}


	sshuser := chunks[0]
	sshaddr := chunks[1]
	if !strings.ContainsRune(sshaddr, ':') {
		sshaddr = fmt.Sprintf("%s:22", sshaddr)
	}

	slog.Info("starting tunnel on", "addr", local, "ssh-user", sshuser, "ssh-addr", sshaddr)


	pkg.TunnelInfo{
		User:    sshuser,
		SshAddr: sshaddr,
		RemoteAddr: remote,
		Agent:      agent,
	}.Start(localConn)

	slog.Info("tunnel stopped")
}
