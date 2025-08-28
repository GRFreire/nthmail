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
	To      []string
	Cc      []string
	Bcc     []string
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
	case strings.EqualFold(s, "html"):
		t = Html
	case strings.EqualFold(s, "md"):
		t = Markdown
	case strings.EqualFold(s, "text"):
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
	m.Subject, _ = dec.DecodeHeader(mail_msg.Header.Get("Subject"))

	to_addrs, _ := mail_msg.Header.AddressList("To")
	m.To = make([]string, len(to_addrs))
	for i, a := range to_addrs {
		m.To[i] = a.Address
	}

	cc_addrs, _ := mail_msg.Header.AddressList("Cc")
	m.Cc = make([]string, len(to_addrs))
	for i, a := range cc_addrs {
		m.Cc[i] = a.Address
	}

	bcc_addrs, _ := mail_msg.Header.AddressList("Bcc")
	m.Bcc = make([]string, len(to_addrs))
	for i, a := range bcc_addrs {
		m.Bcc[i] = a.Address
	}

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

		mail_body, err := Parse_mail_part(mail_msg.Header, txt_bytes)
		if err != nil {
			return m, err
		}

		body.Data = mail_body.Data
		body.MimeType = mail_body.MimeType

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

	body, err := Parse_mail_multipart(mail_msg.Body, params["boundary"])
	if err != nil {
		return m, err
	}

	m.Body = body

	return m, nil
}

type Header interface {
	Get(string) string
}

func Parse_mail_part(header Header, body []byte) (Mail_body, error) {
	content_transfer_encoding := header.Get("Content-Transfer-Encoding")
	content_type := header.Get("Content-Type")

	var mail_body Mail_body

	switch {
	case strings.HasPrefix(content_type, "text/plain"):
		mail_body.MimeType = PlainText
	case strings.HasPrefix(content_type, "text/markdown"):
		mail_body.MimeType = Markdown
	case strings.HasPrefix(content_type, "text/html"):
		mail_body.MimeType = Html
	default:
		return mail_body, errors.New("Content type not supported: " + content_type)
	}

	switch {
	case strings.EqualFold(content_transfer_encoding, "BASE64"):
		decoded_content, err := base64.StdEncoding.DecodeString(string(body))
		if err != nil {
			return mail_body, err
		}

		mail_body.Data = string(decoded_content)

	case strings.EqualFold(content_transfer_encoding, "QUOTED-PRINTABLE"):
		decoded_content, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil {
			return mail_body, err
		}

		mail_body.Data = string(decoded_content)

	default:
		mail_body.Data = string(body)
	}

	return mail_body, nil
}

func Parse_mail_multipart(mime_data io.Reader, boundary string) ([]Mail_body, error) {
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
			body_part, err := Parse_mail_multipart(new_part, params["boundary"])
			if err != nil {
				return body, err
			}

			body = append(body, body_part...)

		} else {

			part_data, err := io.ReadAll(new_part)
			if err != nil {
				return body, err
			}
			part_body, err := Parse_mail_part(new_part.Header, part_data)
			if err != nil {
				return body, err
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
