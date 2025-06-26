package pkg

import (
	"io"
	"log/slog"
	"net"
	"os"
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

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
	srvConn, err := ssh.Dial("tcp", t.SshAddr, &sshConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to ssh server: %v", err)
	}

	remoteConn, err := srvConn.Dial("tcp", t.RemoteAddr)
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


const EnvSSHAuthSock = "SSH_AUTH_SOCK"

func AuthAgent() (ssh.AuthMethod, error) {
	conn, err := net.Dial("unix", os.Getenv(EnvSSHAuthSock))
	if err != nil {
		return nil, err
	}
	client, err := agent.NewClient(conn), err
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeysCallback(client.Signers), nil
}

