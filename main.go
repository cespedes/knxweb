package main

import (
	"log"
	"fmt"
	"flag"
	"time"
	"sort"
	"sync"
	"strings"
	"net/http"
	"encoding/json"
	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
)

const DefaultKNXPort = 3671

type knx_msg struct {
	When  time.Time
	Event knx.GroupEvent
}

var mutex sync.Mutex
var messages []knx_msg
var values = map[cemi.GroupAddr]knx_msg{}
var sorted_values []cemi.GroupAddr

func get_knx_messages(knxrouter string) {
	if !strings.Contains(knxrouter, ":") {
		knxrouter = fmt.Sprintf("%s:%d", knxrouter, DefaultKNXPort)
	}

	client, err := knx.NewGroupTunnel(knxrouter, knx.DefaultTunnelConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	for event := range client.Inbound() {
		msg := knx_msg{When: time.Now(), Event: event}
		mutex.Lock()
		messages = append(messages, msg)
		if _, ok := values[event.Destination]; !ok {
			// this destination has already been seen
			sorted_values = append(sorted_values, event.Destination)
			sort.Slice(sorted_values, func(i, j int) bool { return sorted_values[i] < sorted_values[j]})
		}
		values[event.Destination] = msg
		mutex.Unlock()
		log.Printf("KNX: %+v", event)
		b, _ := json.Marshal(event)
		log.Printf("JSON: %v", string(b))
	}
}

func web_root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ROOT: %s\n", r.URL)
}

func web_get(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]
	var a, b, c uint8
	if path=="latest" {
		mutex.Lock()
		msg := messages[len(messages)-1]
		mutex.Unlock()
		fmt.Fprintf(w, "Last message: %+v", msg)
	} else if path=="all" {
		mutex.Lock()
		for i := range sorted_values {
			fmt.Fprintf(w, "%+v\n", values[sorted_values[i]])
		}
		mutex.Unlock()
	} else if i, _ := fmt.Sscanf(path, "%d/%d/%d", &a, &b, &c) ; i==3 {
		mutex.Lock()
		fmt.Fprintf(w, "Last message to %d/%d/%d: %+v", a, b, c, values[cemi.NewGroupAddr3(a,b,c)])
		mutex.Unlock()
	}
}

func web_set(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "SET: %s", r.URL)
}

func main() {
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	knxrouter := flag.String("knx", "", "address of KNX router")
	flag.Parse()

	go get_knx_messages(*knxrouter)

	http.HandleFunc("/", web_root)
	http.HandleFunc("/get/", web_get)
	http.HandleFunc("/set/", web_set)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
