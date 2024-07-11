# Nothing Mail

A temporary email service

## Getting Started

### Compiling

Requirements:
 - Golang
 - Templ
 - Make

```sh
make
```

### Creating a database:

Requirements:
 - SQLite

```sh
cat migration.sql | sqlite3 db.db
```

### Running:

Available env variables:
 - WEB_SERVER_PORT
 - MAIL_SERVER_PORT
 - MAIL_SERVER_DOMAIN
 - DB_PATH

```sh
./bin/server
```

## TODO

 - Restart when either mail or web server dies
 - Handle attachments
 - Do not store the raw mail data in the DB, maybe use block storage (the provider can be a disk provider at first)
 - Cache subject parsed from email. Then when listing the email it is not necessary to parse all mails and retrieve them.
 - Cache in general?
