package web_server

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/GRFreire/nthmail/pkg/mail_utils"
	"github.com/GRFreire/nthmail/pkg/rig"
	"github.com/go-chi/chi"
	_ "github.com/mattn/go-sqlite3"
	"github.com/microcosm-cc/bluemonday"
)

func Start(db *sql.DB) error {
	server := &ServerResouces{}
	server.db = db

	server.policy = bluemonday.UGCPolicy()
	server.policy.AllowAttrs("style").Globally()

	domain, exists := os.LookupEnv("MAIL_SERVER_DOMAIN")
	if !exists {
		domain = "localhost"
	}
	server.domain = domain

	var port int
	var err error
	port_str, exists := os.LookupEnv("WEB_SERVER_PORT")
	if exists {
		port, err = strconv.Atoi(port_str)
		if err != nil {
			return errors.New("env:WEB_SERVER_PORT is not a number")
		}
	} else {
		port = 3000
	}

	router := server.Routes()

	log.Println("Starting web server at port", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	if err != nil {
		return err
	}

	return nil
}

type ServerResouces struct {
	db     *sql.DB
	policy *bluemonday.Policy
	domain string
}

type db_mail_header struct {
	Id                   int
	Arrived_at           int64
	Rcpt_addr, From_addr string
	Subject              string
}
type db_mail struct {
	Id                   int
	Arrived_at           int64
	Rcpt_addr, From_addr string
	Data                 []byte
}

func (sr ServerResouces) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/", func(res http.ResponseWriter, req *http.Request) {
		page := index_page()
		page.Render(req.Context(), res)
	})

	router.Get("/random", func(res http.ResponseWriter, req *http.Request) {
		inbox_name := rig.GenerateRandomInboxName()
		inbox_addr := fmt.Sprintf("/%s@%s", inbox_name, sr.domain)

		http.Redirect(res, req, inbox_addr, 307)
	})

	router.Get("/{rcpt-addr}", sr.handleInbox)
	router.Get("/{rcpt-addr}/{mail-id}", sr.handleMail)

	return router
}

func (sr ServerResouces) handleInbox(res http.ResponseWriter, req *http.Request) {
	rcpt_addr := chi.URLParam(req, "rcpt-addr")
	if len(rcpt_addr) == 0 {
		res.WriteHeader(404)
		res.Write([]byte("inbox not found"))
		return
	}

	tx, err := sr.db.Begin()
	if err != nil {
		res.WriteHeader(500)
		res.Write([]byte("internal server error"))

		log.Println("could not begin db transaction")
		return
	}
	defer tx.Commit()

	stmt, err := tx.Prepare(
        "SELECT "                  +
            "mails.id, "           +
            "mails.arrived_at, "   +
            "mails.rcpt_addr, "    +
            "mails.from_addr, "    +
            "mails.subject "       +
        "FROM "                    +
            "mails "               +
        "WHERE "                   +
            "mails.rcpt_addr = ? " +
        "ORDER BY "                +
            "mails.arrived_at DESC",
        )
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

	var mails []mail_utils.Mail_obj
	for rows.Next() {
		var m db_mail_header
		err = rows.Scan(&m.Id, &m.Arrived_at, &m.Rcpt_addr, &m.From_addr, &m.Subject)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not scan db row")
			return
		}

		var mail_obj mail_utils.Mail_obj
		mail_obj.Id = m.Id
		mail_obj.Date = time.Unix(m.Arrived_at, 0)
		mail_obj.To = m.Rcpt_addr
		mail_obj.From = m.From_addr
		mail_obj.Subject = m.Subject

		mails = append(mails, mail_obj)
	}

	body := inbox_body(rcpt_addr, mails)
	body.Render(req.Context(), res)
}

func (sr ServerResouces) handleMail(res http.ResponseWriter, req *http.Request) {
	rcpt_addr := chi.URLParam(req, "rcpt-addr")
	if len(rcpt_addr) == 0 {
		res.WriteHeader(404)
		res.Write([]byte("inbox not found"))
		return
	}

	mail_id := chi.URLParam(req, "mail-id")
	if len(rcpt_addr) == 0 {
		res.WriteHeader(404)
		res.Write([]byte("mail not found"))
		return
	}

	tx, err := sr.db.Begin()
	if err != nil {
		res.WriteHeader(500)
		res.Write([]byte("internal server error"))

		log.Println("could not begin db transaction")
		return
	}
	defer tx.Commit()

	stmt, err := tx.Prepare("SELECT mails.id, mails.arrived_at, mails.rcpt_addr, mails.from_addr, mails.data FROM mails WHERE mails.rcpt_addr = ? AND mails.id = ?")
	if err != nil {
		res.WriteHeader(500)
		res.Write([]byte("internal server error"))

		log.Println("could not prepare db stmt")
		return
	}
	defer stmt.Close()

	row := stmt.QueryRow(rcpt_addr, mail_id)

	format, f_pref := mail_utils.Parse_mime_format(req.URL.Query().Get("format"))
	var m db_mail
	err = row.Scan(&m.Id, &m.Arrived_at, &m.Rcpt_addr, &m.From_addr, &m.Data)
	if err != nil {
		res.Write([]byte("404 not found"))

		return
	}

	mail_obj, err := mail_utils.Parse_mail(m.Data, false)
	mail_obj.Date = time.Unix(m.Arrived_at, 0)
	mail_obj.Id = m.Id
	if err != nil {
		res.WriteHeader(500)
		res.Write([]byte("internal server error"))

		log.Println("could not parse mail")
		log.Println(err)
		return
	}

	mail_obj = mail_utils.Set_format_index(mail_obj, format, f_pref)

	body := mail_body_comp(rcpt_addr, mail_obj, sr.policy)
	body.Render(req.Context(), res)
}
