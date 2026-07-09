# Design Spec: Concurrente Client Configuration Healthcheck in `connect`

This design document outlines the implementation of a concurrent healthcheck mechanism in `connect`, triggered by a new `--healthcheck` flag. It tests all configured database connections in parallel (up to a concurrency limit of 5), resolves and tests tunnels cleanly on dynamic local ports, enforces a strict 5-second timeout per connection, and presents a beautiful, barebones final report.

## Requirements

1. **Trigger Flag**: Adding the `--healthcheck` command-line argument to `connect` (i.e. `connect --healthcheck`).
2. **Configuration Inspection**: Retrieve all database configurations defined in `~/.config/connect/config.yaml`.
3. **Concurrency Control**: 
   - Limit parallel tests to a maximum of **5 workers**.
   - Use standard Go concurrency primitives (`sync.WaitGroup`, worker pool pattern, and channels) to orchestrate parallel checks.
4. **Dynamic Port Allocation for Tunnels**:
   - For database connections requiring an SSH tunnel, dynamically find a free local TCP port by listening on `127.0.0.1:0`.
   - Avoid hardcoded ports to prevent race conditions or collisions during concurrent execution.
   - Start the tunnel in a goroutine and close the listener immediately once the health check for that database finishes.
5. **Robust Timeouts**:
   - Enforce a strict **5-second timeout** per database connection/ping using `context.WithTimeout`.
   - Ensure hanging SSH connections or database engines do not block the utility.
6. **Polished, Minimal Output**:
   - Display a single, clear summary of all tested databases at the end.
   - Do not print progress lines from individual goroutines in a tangled way; collect results in memory and print a clean final summary.
   - Format:
     ```
     Checking 5 databases...

     [OK]    prod-maxmara-rw      (Host: 127.0.0.1, Database: maxmara)
     [ERROR] prod-click-ro        (Host: click-gamma.maxmara.it) - connection timed out
     [OK]    dbg-svil-ro          (Host: 127.0.0.1, Database: db_svil)

     Summary: 2 passed, 1 failed, 0 skipped.
     ```
   - No emojis, minimal formatting, in line with global agent preferences.

## Architecture & Data Flow

```
                                  +-----------------------+
                                  | `connect --healthcheck` |
                                  +-----------+-----------+
                                              |
                                              v
                                   [ Load YAML Config ]
                                              |
                                              v
                                    [ Build Task Queue ]
                                              |
                      +-----------------------+-----------------------+
                      |                       |                       |
                      v                       v                       v
                 [ Worker 1 ]            [ Worker 2 ]            [ Worker N ] (Max 5)
                      |                       |                       |
             +--------+--------+              |                       |
             | Tunnel Needed?  |              |                       |
             +--------+--------+              |                       |
                      |                       |                       |
             Yes +----+----+ No               |                       |
                 |         |                  |                       |
                 v         v                  v                       v
          [ Start Tunnel]  [ Ping ]       [ ... ]                 [ ... ]
          [ on port :0  ]  [ direct ]
                 |         |
                 +----+----+
                      |
                      v
             [ Close Tunnel ]
                      |
                      +-----------------------+-----------------------+
                                              |
                                              v
                                     [ Result Channel ]
                                              |
                                              v
                                    [ Collect & Sort ]
                                              |
                                              v
                                    [ Clean Print Output ]
```

## Implementation Details

### Result Struct
```go
type HealthcheckResult struct {
	Alias    string
	Host     string
	Database string
	Success  bool
	Err      error
}
```

### Worker Implementation

Each worker executes a function that:
1. Receives an alias and connection info.
2. Checks if `UserAlias` exists in the `Credentials` map.
3. If a tunnel is specified:
   - Resolves a free local TCP port using `net.Listen("tcp", "127.0.0.1:0")`.
   - Obtains the allocated local port.
   - Starts the SSH tunnel in a separate goroutine.
   - Overrides the database host to `127.0.0.1` and port to the allocated local port.
   - Ensures the listener is closed at the end of the test.
4. Opens the database connection.
5. Performs a `db.PingContext(ctx)` where `ctx` has a 5-second timeout.
6. Returns the result.

### Final Verification and Reports
All results are gathered, sorted alphabetically by alias to maintain deterministic ordering, and displayed to the user.

## Testing Strategy
- Add unit tests verifying that `--healthcheck` gracefully reports failures when config credentials are missing.
- Verify that a dummy or mocked database can be checked successfully, or that a database ping handles timeout/connection error states correctly.
- Run complete Go tests suite to ensure zero regressions.
