package main

import (
	"database/sql"
	"io"
	"log"
	"time"

	"github.com/emersion/go-smtp"
	_ "github.com/mattn/go-sqlite3"
)

type Backend struct {
	db *sql.DB
}

func (backend *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	tx, err := backend.db.Begin()
	if err != nil {
		return nil, err
	}

	return &Session{
		tx: tx,
	}, nil
}

type Session struct {
	tx           *sql.Tx
	from, rcpt   string
	rcpt_addr_id int64
}

func (session *Session) AuthPlain(username, password string) error {
	return nil
}

func (session *Session) Mail(from string, opts *smtp.MailOptions) error {
	session.from = from
	return nil
}

func (session *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	session.rcpt = to

	stmt, err := session.tx.Prepare("INSERT OR IGNORE INTO inboxes (addr) VALUES (?) RETURNING id")
	if err != nil {
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(to)
	if err != nil {
		return err
	}

    // sometimes does not work and returns 0
    // see https://github.com/mattn/go-sqlite3/issues/1140
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	log.Println("Last id: ", id)

	session.rcpt_addr_id = id

	return nil
}

func (session *Session) Data(reader io.Reader) error {
	if bytes, err := io.ReadAll(reader); err != nil {
		return err
	} else {

		stmt, err := session.tx.Prepare("INSERT INTO mails (inbox_id, from_addr, data) VALUES (?, ?, ?)")
		if err != nil {
			return err
		}

		_, err = stmt.Exec(session.rcpt_addr_id, session.from, bytes)
		if err != nil {
			return err
		}

		err = session.tx.Commit()
		if err != nil {
			return err
		}

		log.Println("New mail from <", session.from, "> to <", session.rcpt, ">")
		log.Println("Data:", string(bytes))

	}
	return nil
}

func (session *Session) Reset() {}

func (session *Session) Logout() error {
	return nil
}

func main() {
	db, err := sql.Open("sqlite3", "./db.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	backend := &Backend{
		db: db,
	}

	server := smtp.NewServer(backend)

	server.Addr = "localhost:1025"
	server.Domain = "localhost"
	server.WriteTimeout = 60 * time.Second
	server.ReadTimeout = 60 * time.Second
	server.MaxMessageBytes = 1024 * 1024
	server.MaxRecipients = 50
	server.AllowInsecureAuth = true

	log.Println("Starting server at", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
