package main

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"github.com/GRFreire/nthmail/pkg/rig"
	"github.com/go-chi/chi"
	_ "github.com/mattn/go-sqlite3"
)

type db_mail struct {
	Id                   int
	Arrived_at           int64
	Rcpt_addr, From_addr string
	Data                 []byte
}

type MIMEType uint8
type MediaType uint8

const (
	PlainText MIMEType = iota
	Html
	Markdown
)

const (
	NotMultipart MediaType = iota
	Alternative
	Mixed
)

type mail_body struct {
	MimeType MIMEType
	Data     string
}

type mail_obj struct {
	Id      int
	From    string
	Date    string
	To      string
	Bcc     string
	Subject string

	Body []mail_body
	MediaType
	PreferedBodyIndex int
}

func parse_mime_format(s string) (MIMEType, bool) {
	var t MIMEType

	if s == "" {
		return t, false
	}

	switch {
	case strings.Compare(s, "html") == 0:
		t = Html
	case strings.Compare(s, "md") == 0:
		t = Markdown
	case strings.Compare(s, "text") == 0:
		t = PlainText
	default:
		return t, false
	}

	return t, true
}

func parse_mail(dbm db_mail, header_only bool) (mail_obj, error) {
	var m mail_obj
	m.Id = dbm.Id

	mail_msg, err := mail.ReadMessage(bytes.NewReader(dbm.Data))
	if err != nil {
		return m, errors.New("Could not read message")
	}

	// HEADERS
	dec := new(mime.WordDecoder)
	m.From, _ = dec.DecodeHeader(mail_msg.Header.Get("From"))
	m.Date, _ = dec.DecodeHeader(mail_msg.Header.Get("Date"))
	m.To, _ = dec.DecodeHeader(mail_msg.Header.Get("To"))
	m.Bcc, _ = dec.DecodeHeader(mail_msg.Header.Get("Bcc"))
	m.Subject, _ = dec.DecodeHeader(mail_msg.Header.Get("Subject"))

	if header_only {
		return m, nil
	}

	content_type := mail_msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(content_type)
	if err != nil {
		return m, err
	}

	if content_type == "" || !strings.HasPrefix(mediaType, "multipart/") {
		var txt []byte
		_, err := mail_msg.Body.Read(txt)
		if err != nil {
			return m, err
		}

		var body mail_body
		body.MimeType = PlainText
		body.Data = string(txt)

		m.MediaType = NotMultipart
		m.Body = append(m.Body, body)

		return m, nil
	}

	if mediaType == "multipart/mixed" {
		m.MediaType = Mixed
	} else if mediaType == "multipart/alternative" {
		m.MediaType = Alternative
	} else {
		return m, errors.New("Not supported multipart type")
	}

	body, err := parse_mail_part(mail_msg.Body, params["boundary"])
	if err != nil {
		return m, err
	}

	m.Body = body

	return m, nil
}

func parse_mail_part(mime_data io.Reader, boundary string) ([]mail_body, error) {
	var body []mail_body

	reader := multipart.NewReader(mime_data, boundary)
	if reader == nil {
		return body, nil
	}

	for {
		new_part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		if err != nil {
			return body, err
		}

		mediaType, params, err := mime.ParseMediaType(new_part.Header.Get("Content-Type"))

		if err != nil {
			return body, err
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			body_part, err := parse_mail_part(new_part, params["boundary"])
			if err != nil {
				return body, err
			}

			body = append(body, body_part...)

		} else {

			part_data, err := io.ReadAll(new_part)
			if err != nil {
				return body, err
			}
			content_transfer_encoding := new_part.Header.Get("Content-Transfer-Encoding")
			content_type := new_part.Header.Get("Content-Type")

			var part_body mail_body

			switch {
			case strings.HasPrefix(content_type, "text/plain"):
				part_body.MimeType = PlainText
			case strings.HasPrefix(content_type, "text/markdown"):
				part_body.MimeType = Markdown
			case strings.HasPrefix(content_type, "text/html"):
				part_body.MimeType = Html
			default:
				return body, errors.New("Content type not supported: " + content_type)
			}

			switch {
			case strings.Compare(content_transfer_encoding, "BASE64") == 0:
				decoded_content, err := base64.StdEncoding.DecodeString(string(part_data))
				if err != nil {
					return body, err
				}

				part_body.Data = string(decoded_content)

			case strings.Compare(content_transfer_encoding, "QUOTED-PRINTABLE") == 0:
				decoded_content, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(part_data)))
				if err != nil {
					return body, err
				}

				part_body.Data = string(decoded_content)

			default:
				part_body.Data = string(part_data)
			}

			body = append(body, part_body)

		}
	}

	return body, nil
}

func set_format_index(m mail_obj, format MIMEType, pref bool) mail_obj {
	m.PreferedBodyIndex = -1
	for i, b := range m.Body {
		if pref && b.MimeType == format {
			m.PreferedBodyIndex = i
			break
		}

		if m.PreferedBodyIndex == -1 {
			m.PreferedBodyIndex = i
			continue
		}

		if b.MimeType == Html {
			m.PreferedBodyIndex = i
			continue
		}

		if b.MimeType == Markdown {
			if m.Body[i].MimeType != Html {
				m.PreferedBodyIndex = i
			}
		}
	}

	return m
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

		var mails []mail_obj
		for rows.Next() {
			var m db_mail
			err = rows.Scan(&m.Id, &m.Arrived_at, &m.Rcpt_addr, &m.From_addr, &m.Data)
			if err != nil {
				res.WriteHeader(500)
				res.Write([]byte("internal server error"))

				log.Println("could not scan db row")
				return
			}

			mail_obj, err := parse_mail(m, true)
			if err != nil {
				res.WriteHeader(500)
				res.Write([]byte("internal server error"))

				log.Println("could not parse mail")
				log.Println(err)
				return
			}

			mails = append(mails, mail_obj)
		}

		body := inbox_body(rcpt_addr, mails)
		body.Render(req.Context(), res)
	})

	router.Get("/{rcpt-addr}/{mail-id}", func(res http.ResponseWriter, req *http.Request) {
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

		tx, err := db.Begin()
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not begin db transaction")
			return
		}

		stmt, err := tx.Prepare("SELECT mails.id, mails.arrived_at, mails.rcpt_addr, mails.from_addr, mails.data FROM mails WHERE mails.rcpt_addr = ? AND mails.id = ?")
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not prepare db stmt")
			return
		}
		defer stmt.Close()

		row := stmt.QueryRow(rcpt_addr, mail_id)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not query db stmt")
			return
		}

		format, f_pref := parse_mime_format(req.URL.Query().Get("format"))
		var m db_mail
		err = row.Scan(&m.Id, &m.Arrived_at, &m.Rcpt_addr, &m.From_addr, &m.Data)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not scan db row")
			return
		}

		mail_obj, err := parse_mail(m, false)
		if err != nil {
			res.WriteHeader(500)
			res.Write([]byte("internal server error"))

			log.Println("could not parse mail")
			log.Println(err)
			return
		}

		mail_obj = set_format_index(mail_obj, format, f_pref)

		body := mail_body_comp(rcpt_addr, mail_obj)
		body.Render(req.Context(), res)
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
