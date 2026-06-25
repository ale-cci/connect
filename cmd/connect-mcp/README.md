# Connect MCP Server

A Model Context Protocol (MCP) server for secure database querying and schema introspection by LLMs.

## Usage

You can run `connect-mcp` in either stdio mode or HTTP Server-Sent Events (SSE) mode.

- **Default (stdio mode):**
  ```bash
  connect-mcp <alias>
  ```
- **HTTP SSE mode:**
  ```bash
  connect-mcp -http :8000 <alias>
  ```

Replace `<alias>` with one of your pre-configured databases defined in `~/.config/connect/config.yaml`.

## Claude Desktop Integration

To register `connect-mcp` with Claude Desktop, add the following to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "connect-mcp": {
      "command": "connect-mcp",
      "args": ["sales_prod"]
    }
  }
}
```

Make sure the `connect-mcp` binary is available in your system's `PATH`.

## Registered Tools

`connect-mcp` exposes the following tools to the LLM client:

- `list_tables` - Lists all tables in the connected database.
- `describe_table` - Fetches columns, types, keys, and default values for a table.
- `execute_query` - Securely executes read or write SQL queries, returning tabular JSON output.
