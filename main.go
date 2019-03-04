package main

import (
	"log"
	"fmt"
	"flag"
	"net/http"
//	"github.com/vapourismo/knx-go/knx"
)

func webserver(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "this is the web server")
}

func main() {
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	flag.Parse()

	http.HandleFunc("/", webserver)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
