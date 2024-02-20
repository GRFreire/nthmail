package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/GRFreire/nthmail/pkg/rig"
	"github.com/go-chi/chi"
	_ "github.com/mattn/go-sqlite3"
)

type mail struct {
	Id                   int
	Arrived_at           int
	Rcpt_addr, From_addr string
	Data                 []byte
}

func main() {
	db, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	domain, exists := os.LookupEnv("MAIL_SERVER_DOMAIN")
	if !exists {
		domain = "localhost"
	}

	router := chi.NewRouter()

	router.Get("/", func(res http.ResponseWriter, req *http.Request) {
		page := index_page()
		page.Render(req.Context(), res)
	})

	router.Get("/random", func(res http.ResponseWriter, req *http.Request) {
		inbox_name := rig.GenerateRandomInboxName()
		inbox_addr := fmt.Sprintf("/%s@%s", inbox_name, domain)

		http.Redirect(res, req, inbox_addr, 307)
	})

	router.Get("/{rcpt-addr}", func(res http.ResponseWriter, req *http.Request) {
		rcpt_addr := chi.URLParam(req, "rcpt-addr")
		if len(rcpt_addr) == 0 {
			res.WriteHeader(404)
			res.Write([]byte("inbox not found"))
			return
		}

		tx, err := db.Begin()
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not begin db transaction")
			return
		}

		stmt, err := tx.Prepare("SELECT mails.id, mails.arrived_at, mails.rcpt_addr, mails.from_addr, mails.data FROM mails WHERE mails.rcpt_addr = ?")
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not prepare db stmt")
			return
		}
		defer stmt.Close()

		rows, err := stmt.Query(rcpt_addr)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not query db stmt")
			return
		}
		defer rows.Close()

		var mails []mail
		for rows.Next() {
			var m mail
			err = rows.Scan(&m.Id, &m.Arrived_at, &m.Rcpt_addr, &m.From_addr, &m.Data)
			if err != nil {
				res.WriteHeader(500)
				res.Write([]byte("internal server error"))

				log.Println("could not scan db row")
				return
			}

			mails = append(mails, m)
		}
		b, err := json.Marshal(mails)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not marshal json")
			return
		}

		res.Write([]byte(b))
	})

	var port int
	port_str, exists := os.LookupEnv("WEB_SERVER_PORT")
	if exists {
		port, err = strconv.Atoi(port_str)
		if err != nil {
			log.Fatal("env:MAIL_SERVER_PORT is not a number")
		}
	} else {
		port = 3000
	}

	log.Println("Listening on port", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	if err != nil {
		log.Fatal(err)
	}
}
