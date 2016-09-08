package main

import (
	"io"
	"log"
	"net/http"

	"github.com/GeertJohan/go.rice"
)

// replyJSON serves the in-memory `D3JSON` JSON string to the frontend graph.
func replyJSON(res http.ResponseWriter, req *http.Request) {
	io.WriteString(res, D3JSON)
}

// Listen configures and starts a web server, enclosing it in an asynchronous goroutine.
func Listen(port string) {
	go func() {
		http.Handle("/", http.FileServer(rice.MustFindBox("public").HTTPBox()))
		http.HandleFunc("/hubdata.json", replyJSON)
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			log.Fatalf("Unable to start web server: %s", err.Error())
		}
	}()
}
