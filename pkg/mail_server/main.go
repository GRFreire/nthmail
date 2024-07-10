package mail_server

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
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
	tx         *sql.Tx
	from, rcpt string
	arrived_at int64
}

func (session *Session) AuthPlain(username, password string) error {
	return nil
}

func (session *Session) Mail(from string, opts *smtp.MailOptions) error {
	session.arrived_at = time.Now().UTC().Unix()

	session.from = from
	return nil
}

func (session *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	session.rcpt = to

	return nil
}

func (session *Session) Data(reader io.Reader) error {
	defer session.tx.Rollback()
	if bytes, err := io.ReadAll(reader); err != nil {
		return err
	} else {

		stmt, err := session.tx.Prepare("INSERT INTO mails (arrived_at, rcpt_addr, from_addr, data) VALUES (?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(session.arrived_at, session.rcpt, session.from, bytes)
		if err != nil {
			return err
		}

		err = session.tx.Commit()
		if err != nil {
			return err
		}

	}
	return nil
}

func (session *Session) Reset() {}

func (session *Session) Logout() error {
	return nil
}

func Start(db *sql.DB) error {
	backend := &Backend{
		db: db,
	}

	server := smtp.NewServer(backend)

	domain, exists := os.LookupEnv("MAIL_SERVER_DOMAIN")
	if !exists {
		domain = "localhost"
	}

	var port int
	var err error
	port_str, exists := os.LookupEnv("MAIL_SERVER_PORT")
	if exists {
		port, err = strconv.Atoi(port_str)
		if err != nil {
			return errors.New("env:MAIL_SERVER_PORT is not a number")
		}
	} else {
		port = 1025
	}

	server.Addr = fmt.Sprintf("%s:%d", domain, port)
	server.Domain = domain
	server.WriteTimeout = 60 * time.Second
	server.ReadTimeout = 60 * time.Second
	server.MaxMessageBytes = 1024 * 1024
	server.MaxRecipients = 50
	server.AllowInsecureAuth = true

	log.Println("Starting server at", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
