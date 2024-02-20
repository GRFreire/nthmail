all: mail web

mail:
	go build -o "./bin/$@_server" "./cmd/mail_server"

web:
	templ generate
	go build -o "./bin/$@_server" "./cmd/web_server"
