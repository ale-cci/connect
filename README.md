# Connect

> A secure, signal-resilient interactive SQL CLI client, automatic SSH tunneling manager, and Model Context Protocol (MCP) server for Go.

[![Go Report Card](https://goreportcard.com/badge/codeberg.org/ale-cci/connect)](https://goreportcard.com/report/codeberg.org/ale-cci/connect)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ale-cci/connect?filename=go.mod)](go.mod)

[Features](#features) • [Installation](#installation) • [Configuration](#configuration) • [Components](#components) • [Shell Completions](#shell-completions)

---

## Features

*   **Interactive SQL CLI (`connect`):** A custom raw-mode terminal client with full history navigation (`Ctrl+P`/`Ctrl+N`/`Ctrl+R`), signal resilience, dynamic table rendering with multi-line cell wrapping, and `Ctrl+Z` background suspend/resume support.
*   **LLM Database Server (`connect-mcp`):** Exposes your databases to LLM clients (such as Claude Desktop or Cursor) via the Model Context Protocol (MCP) using stdio or HTTP SSE.
*   **Seamless SSH Tunneling:** Automatic SSH tunneling with ssh-agent integration allows you to query remote databases securely without managing manual port forwards.
*   **Bulk Connections Import (`connect-manager`):** Batch import database connections directly from CSV files into your central YAML configuration.
*   **Standalone Forwarding (`tunnel`):** A lightweight utility to spin up independent local SSH port/socket forwards on demand.

---

## Installation

Ensure you have Go 1.22+ installed, then run the installation commands for the components you need:

```bash
# Install the interactive SQL database CLI
go install codeberg.org/ale-cci/connect/cmd/connect@master

# Install the Model Context Protocol (MCP) server
go install codeberg.org/ale-cci/connect/cmd/connect-mcp@master

# Install the database connections manager (CSV importer)
go install codeberg.org/ale-cci/connect/cmd/connect-manager@master

# Install the standalone tunneling manager
go install codeberg.org/ale-cci/connect/cmd/tunnel@master
```

> [!TIP]
> **Pre-release / Beta Version:** If you want to install the latest pre-release or beta versions of these tools, simply replace `@master` with `@beta` in the commands above (e.g., `go install ...@beta`).

---

## Configuration

Both `connect` and `connect-mcp` load settings and credentials from a YAML configuration file located at:
`~/.config/connect/config.yaml`

### Configuration Schema

Create the directory and configuration file:

```yaml
# Mapping of reusable user credentials
credentials:
  prod_admin:
    username: admin
    password: SuperSecurePassword123
  dev_user:
    username: developer
    password: DevPassword456

# Database connection mappings
databases:
  sales_prod:
    host: 10.0.1.5
    port: 3306
    alias: prod_admin
    database: sales
    driver: mysql
    tag:
      - production
    # Automatically spins up an SSH tunnel in the background using your local ssh-agent
    tunnel: tunnel-user@ssh-jump-host.internal

  local_db:
    host: /var/run/mysqld/mysqld.sock
    port: 0                   # Setting port to 0 forces a UNIX socket connection
    alias: dev_user
    database: dev_schema
    driver: mysql
    tag:
      - local

# Client options
options:
  autolimit: 100              # Automatically appends LIMIT to select queries (0 to disable)
  histsize: 2000              # Maximum command history size
  tabsize: 4                  # Spaces per tab in the client display
```

> [!TIP]
> **SSH Agent Requirement:** To use the automatic SSH tunneling feature (`tunnel`), you must have a running local SSH agent containing your key (e.g., loaded via `ssh-add`).

> [!NOTE]
> **UNIX Sockets:** Specifying a file path for `host` and setting `port` to `0` forces the MySQL driver to connect via local UNIX sockets, bypassing TCP entirely.

---

## Components

Connect is split into modular tools. Each subdirectory has its own dedicated, in-depth documentation:

*   [**`connect` (Interactive CLI)**](./cmd/connect/README.md): Detailed mechanics of the interactive SQL shell, query navigation shortcuts, vertical alignment tables, and custom slash commands (`\config`, `\schema`, etc.).
*   [**`connect-mcp` (Model Context Protocol Server)**](./cmd/connect-mcp/README.md): Step-by-step Claude Desktop integration, STDIO vs HTTP SSE configurations, and LLM tools description.
*   [**`connect-manager` (Bulk Configuration)**](./cmd/connect-manager/README.md): Format of the CSV file used to import/manage connection metadata and alias bindings.
*   [**`tunnel` (Standalone Port-Forwarding)**](./cmd/tunnel/README.md): Standalone command usage for tunneling Unix/TCP connections over SSH.

---

## Shell Completions

You can enable auto-completion of database aliases for the `connect` and `connect-mcp` commands.

### Bash
Add the following line to your `~/.bashrc`:
```bash
source /path/to/connect/shell-completions/connect-completion.bash
```

### Zsh
Add the following line to your `~/.zshrc`:
```zsh
source /path/to/connect/shell-completions/connect-completion.zsh
```
