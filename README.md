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
# TODO

- [ ] funzione per effettuare dump
- [x] importazione di file csv
- [ ] display di stringhe multiriga
- [x] gestione di tunnel
- [x] valutare anche singoli apici come carattere di escaping
- [ ] in caso di typo mostrare N alias più simili / help per mostrare elenco alias

# Implementato grazie a

- https://elliotchance.medium.com/how-to-create-an-ssh-tunnel-in-go-b63722d682aa
- https://ixday.github.io/post/golang_ssh_tunneling/

