package main

import (
	"log"
	"fmt"
	"flag"
	"strings"
	"net/http"
	"github.com/vapourismo/knx-go/knx"
)

const DefaultKNXPort = 3671

func webserver(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "this is the web server")
}

func main() {
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	knxrouter := flag.String("knx", "", "address of KNX router")
	flag.Parse()

	if !strings.Contains(*knxrouter, ":") {
			*knxrouter = fmt.Sprintf("%s:%d", *knxrouter, DefaultKNXPort)
	}
	client, err := knx.NewGroupTunnel(*knxrouter, knx.DefaultTunnelConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	for msg := range client.Inbound() {
		log.Printf("KNX: %+v", msg)
	}

	http.HandleFunc("/", webserver)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
