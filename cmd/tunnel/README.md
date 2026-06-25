# Connect Tunnel

A standalone SSH tunneling manager for port forwarding.

## Usage

```bash
tunnel -local <addr> -ssh <user@host:port> -remote <remote_addr_or_path>
```

## Command-Line Flags

- `-local` (default `127.0.0.1:1234`): The local TCP address to listen on.
- `-ssh` (default `user@host.addr:22`): SSH connection details (jump host).
- `-remote` (default `/var/lib/docker.sock`): Target destination (TCP address or UNIX socket path on the remote server).

## Prerequisites

- **SSH Agent:** A running SSH agent on the local machine with loaded credentials (configured and loaded via `ssh-add`).
