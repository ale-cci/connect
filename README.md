# Installation
```
go install codeberg.org/ale-cci/connect/cmd/connect@latest
go install codeberg.org/ale-cci/connect/cmd/connect-manager@latest
```

# Setup
```
$ connect-manager import connection-file.csv
$ connect database-alias
$ connect-dump database-alias blueprint.yaml -p param1=value1 -p param2=value2 -o file.sql
$ connect-dump database-alias blueprint.yaml --params=params.json -o file.sql
```

# Possible next features
- [x] load configurations from csv file
- [x] ssh tunnel management
- [x] Reverse history search
- [x] save history to file

- [ ] correctly display tables with multiline strings
- [ ] query autocompletion

- [x] custom commands
    - \config set rowlimit 100
    - \config set tabsize 4
    - \config get tabsize
    - \config get

- [ ] more commands
    - \export filename.sql <blueprint>
    - !! expands to previous query


- [ ] Build showing the current git version
- [ ] syntax highlight
- [ ] docs
- [ ] Release history

#### useful links
- https://elliotchance.medium.com/how-to-create-an-ssh-tunnel-in-go-b63722d682aa
- https://ixday.github.io/post/golang_ssh_tunneling/
- https://cs.opensource.google/go/x/term/+/refs/tags/v0.32.0:term_unix_bsd.go
