package mail_server

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GRFreire/nthmail/pkg/mail_utils"
	"github.com/emersion/go-smtp"
	_ "github.com/mattn/go-sqlite3"
)

type Backend struct {
	db     *sql.DB
	domain string
}

func (backend *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	tx, err := backend.db.Begin()
	if err != nil {
		return nil, err
	}

	return &Session{
		tx:     tx,
		domain: backend.domain,
	}, nil
}

type Session struct {
	tx         *sql.Tx
	from, rcpt string
	arrived_at int64
	domain     string
}

func get_addr_domain(addr string) string {
	index := strings.Index(addr, "@")
	if index < 0 {
		return ""
	}

	return addr[index+1:]
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
	if get_addr_domain(to) != session.domain {
		return errors.New("To addr domain is not available in this server")
	}
	session.rcpt = to

	return nil
}

func (session *Session) Data(reader io.Reader) error {
	defer session.tx.Rollback()

	bytes, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	mail_obj, err := mail_utils.Parse_mail(bytes, true)
	if err != nil {
		return err
	}

	if get_addr_domain(mail_obj.To) != session.domain {
		return errors.New("To addr domain is not available in this server")
	}

	stmt, err := session.tx.Prepare("INSERT INTO mails (arrived_at, rcpt_addr, from_addr, subject, data) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		println(err)
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(session.arrived_at, session.rcpt, mail_obj.From, mail_obj.Subject, bytes)
	if err != nil {
		return err
	}

	err = session.tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (session *Session) Reset() {}

func (session *Session) Logout() error {
	return nil
}

func Start(db *sql.DB) error {
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

	backend := &Backend{
		db:     db,
		domain: domain,
	}

	server := smtp.NewServer(backend)

	server.Addr = fmt.Sprintf(":%d", port)
	server.Domain = domain
	server.WriteTimeout = 60 * time.Second
	server.ReadTimeout = 60 * time.Second
	server.MaxMessageBytes = 1024 * 1024
	server.MaxRecipients = 50
	server.AllowInsecureAuth = true

	log.Println("Starting mail server at", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
