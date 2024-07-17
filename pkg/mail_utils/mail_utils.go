package mail_utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"slices"
	"strings"
	"time"
)

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

type Mail_body struct {
	MimeType MIMEType
	Data     string
}

type Mail_obj struct {
	Id      int
	From    string
	Date    time.Time
	To      string
	Bcc     string
	Subject string

	Body []Mail_body
	MediaType
	PreferedBodyIndex int
}

func Parse_mime_format(s string) (MIMEType, bool) {
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

func Parse_mail(m_data []byte, header_only bool) (Mail_obj, error) {
	var m Mail_obj

	mail_msg, err := mail.ReadMessage(bytes.NewReader(m_data))
	if err != nil {
		return m, errors.New("Could not read message")
	}

	// HEADERS
	dec := new(mime.WordDecoder)
	m.From, _ = dec.DecodeHeader(mail_msg.Header.Get("From"))
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
		txt_bytes, err := io.ReadAll(mail_msg.Body)
		if err != nil {
			return m, err
		}

		var body Mail_body
		body.MimeType = PlainText
		body.Data = string(txt_bytes)

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

	body, err := Parse_mail_part(mail_msg.Body, params["boundary"])
	if err != nil {
		return m, err
	}

	m.Body = body

	return m, nil
}

func Parse_mail_part(mime_data io.Reader, boundary string) ([]Mail_body, error) {
	var body []Mail_body

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
			body_part, err := Parse_mail_part(new_part, params["boundary"])
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

			var part_body Mail_body

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

func Set_format_index(m Mail_obj, format MIMEType, pref bool) Mail_obj {
	priority := []MIMEType{Html, Markdown, PlainText}

	m.PreferedBodyIndex = -1
	curr_p := len(priority)
	for i, b := range m.Body {
		if pref && format == b.MimeType {
			m.PreferedBodyIndex = i
			break
		}

		p := slices.Index(priority, b.MimeType)
		if p < curr_p {
			curr_p = p
			m.PreferedBodyIndex = i
		}
	}

	return m
}
