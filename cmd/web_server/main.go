package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi"
)

func main() {
	router := chi.NewRouter()

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello world"))
	})

	var port int
    var err error
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
    http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}
