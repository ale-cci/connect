# Installation
```
go install codeberg.org/ale-cci/connect/cmd/connect@latest
go install codeberg.org/ale-cci/connect/cmd/connect-manager@latest
```

# Setup
```
$ connect-manager import connection-file.csv
$ connect database-alias
```

# Possible next features
- [x] load configurations from csv file
- [x] ssh tunnel management

- [ ] Reverse history search
- [ ] correctly display tables with multiline strings
- [ ] Build showing the current git version
- [ ] save history to file
- [ ] query autocompletion

- [ ] custom commands
    - /set rowlimit 100
    - /set tabsize 4
    - /save tabsize
    - /save all
    - /show tabsize
    - /dump filename.xyz select xyz from tablename
    - !! expands to previous query

- [ ] syntax highlight
- [ ] docs
- [ ] Release history

#### useful links
- https://elliotchance.medium.com/how-to-create-an-ssh-tunnel-in-go-b63722d682aa
- https://ixday.github.io/post/golang_ssh_tunneling/
- https://cs.opensource.google/go/x/term/+/refs/tags/v0.32.0:term_unix_bsd.go
