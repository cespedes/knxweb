package main

import (
	"log"
	"fmt"
	"net/http"
//	"github.com/vapourismo/knx-go/knx"
)

func webserver(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "this is the web server")
}

func main() {
	http.HandleFunc("/", webserver)
	log.Fatal(http.ListenAndServe(":8001", nil))
}
