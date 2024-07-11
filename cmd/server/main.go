package main

import (
	"database/sql"
	"github.com/GRFreire/nthmail/pkg/mail_server"
	"github.com/GRFreire/nthmail/pkg/web_server"
	"log"
	"os"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath, exists := os.LookupEnv("DB_PATH")
	if !exists {
		dbPath = "./db.db"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	log.Println("Openning sqlite db at", dbPath)

	var wg sync.WaitGroup
	wg.Add(1)
	go func(db *sql.DB) {
		defer wg.Done()
		err = mail_server.Start(db)
		if err != nil {
			log.Fatal(err)
		}
	}(db)

	wg.Add(1)
	go func(db *sql.DB) {
		defer wg.Done()
		err = web_server.Start(db)
		if err != nil {
			log.Fatal(err)
		}
	}(db)

	wg.Wait()
}
