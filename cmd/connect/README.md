# Connect CLI Client

An interactive, signal-resilient SQL CLI client for MySQL.

## Usage

```bash
connect <alias>
```

Replace `<alias>` with one of your pre-configured databases defined in `~/.config/connect/config.yaml`.

## Key Interactive Terminal Features

`connect` runs inside a fully custom raw-mode terminal interface, granting elite console mechanics:

- **History Navigation:** Use `Ctrl+P` (Previous) and `Ctrl+N` (Next) to traverse historical queries.
- **Reverse Search:** Press `Ctrl+R` to search backwards through history, and hit `Enter` to load the query.
- **Ctrl+Z / Background Support:** Press `Ctrl+Z` to suspend the CLI and return to your shell. Run `fg` to resume the session—retaining the raw terminal state, active command buffer, and cursor position exactly where you left it.
- **Smart Tabular Display:** Query results containing newlines (`\n`) are neatly formatted, vertically aligned, with boundaries and cell grids fully intact.

## Custom Console Slash Commands

In addition to standard SQL queries, you can execute client-specific commands:

- `\help` - Display available commands.
- `\config get` - View runtime settings.
- `\config set <name> <value>` - Dynamically change options on the fly (e.g., `\config set autolimit 50`).
- `\schema` - Saves the active database schema information.
