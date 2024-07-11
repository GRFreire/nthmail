FROM golang:1.22 as build

RUN apt-get update && apt-get upgrade -y
RUN apt-get install make sqlite3 -y

WORKDIR /app
ENV DB_PATH=/data/db.db

RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . .
RUN go mod download
RUN make -B

CMD ["./bin/server"]
