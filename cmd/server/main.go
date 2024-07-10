package main

import (
	"database/sql"
	"github.com/GRFreire/nthmail/pkg/mail_server"
	"github.com/GRFreire/nthmail/pkg/web_server"
	"log"
    "sync"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

    var wg sync.WaitGroup
    wg.Add(1)
    go func(db *sql.DB) {
        defer wg.Done()
        mail_server.Start(db)
    }(db)

    wg.Add(1)
    go func(db *sql.DB) {
        defer wg.Done()
        web_server.Start(db)
    }(db)

    wg.Wait()
}
