# Connect Manager

A utility for importing and managing database connections in bulk.

## Usage

```bash
connect-manager import <csv-file>
```

## How It Works

- **CSV Parsing:** `connect-manager` reads a CSV file containing connection properties and automatically parses them.
- **Config Management:** It writes and updates connection information directly in your centralized YAML configuration file at `~/.config/connect/config.yaml`.

## CSV Schema

The input CSV file must have the following header names:

`alias`, `host`, `port`, `database`, `tunnel`, `user`, `driver`

### Example CSV Content

Below is an example of a compatible CSV import file:

```csv
alias,host,port,database,tunnel,user,driver
sales_prod,10.0.1.5,3306,sales,,prod_admin,mysql
sales_staging,10.0.2.5,3306,sales,tunnel-user@ssh-jump-host.internal,prod_admin,mysql
local_dev,/var/run/mysqld/mysqld.sock,0,dev_schema,,dev_user,mysql
```
