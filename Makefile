all:
	templ generate
	go build -o ./bin/server ./cmd/server

