package pkg

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Address struct {
	Net string
	Addr string
}

func addrFromString(addr string) Address {
	if strings.Contains(addr, "/") {
		return Address { Net: "unix", Addr: addr }
	}
	return Address { Net: "tcp",  Addr: addr }
}

type TunnelInfo struct {
	User string

	SshAddr string
	RemoteAddr string
	LocalAddr  string

	Agent ssh.AuthMethod
}

func (t TunnelInfo) Start(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		err = t.forward(conn)
		if err != nil {
			slog.Error("forwarding failed", "err", err)
		}
	}
}

func (t TunnelInfo) forward(localConn net.Conn) error {
	sshConfig := ssh.ClientConfig{
		User: t.User,
		Auth: []ssh.AuthMethod{t.Agent},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshAddr := addrFromString(t.SshAddr)

	srvConn, err := ssh.Dial(sshAddr.Net, sshAddr.Addr, &sshConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to ssh server: %v", err)
	}

	remoteAddr := addrFromString(t.RemoteAddr)
	remoteConn, err := srvConn.Dial(remoteAddr.Net, remoteAddr.Addr)
	if err != nil {
		return fmt.Errorf("unable to connect to remote addr: %v", err)
	}

	copyBytes := func(writer, reader net.Conn) {
		_, err := io.Copy(writer, reader)
		if err != nil {
			slog.Error("copy error", "err", err)
		}
	}

	go copyBytes(localConn, remoteConn)
	go copyBytes(remoteConn, localConn)
	return nil
}



func AuthAgent() (ssh.AuthMethod, error) {
	conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil, err
	}
	client, err := agent.NewClient(conn), err
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(client.Signers), nil
}

