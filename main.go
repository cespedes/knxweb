package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
)

const (
	KNXDefaultPort = 3671
	KNXTimeout     = 10 // no messages in 10 seconds: probable error in connection
)

type knxMsg struct {
	When  time.Time
	Event knx.GroupEvent
}

type addrNameType struct {
	Name string
	Type dpt.DatapointValue
}

var devices = make(map[cemi.IndividualAddr]string)
var addresses = make(map[cemi.GroupAddr]addrNameType)

func (k knxMsg) String() string {
	s := k.When.Format("2006-01-02 15:04:05")
	switch k.Event.Command {
	case knx.GroupRead:
		s += " read:"
	case knx.GroupResponse:
		s += " response:"
	case knx.GroupWrite:
		s += " write:"
	default:
		s += " ???:"
	}
	s += " " + k.Event.Source.String() + " " + k.Event.Destination.String() + "=" + fmt.Sprint(k.Event.Data)
	if str, ok := devices[k.Event.Source]; ok {
		s += " " + str
	}
	if nt, ok := addresses[k.Event.Destination]; ok {
		if err := addresses[k.Event.Destination].Type.Unpack(k.Event.Data); err != nil {
			fmt.Printf("Network: Error parsing %v for %v\n", k.Event.Data, k.Event.Destination)
		} else {
			s += " " + nt.Name + "=" + fmt.Sprint(nt.Type)
		}
	}
	return s
}

var mutex sync.Mutex
var messages []knxMsg
var values = map[cemi.GroupAddr]knxMsg{}
var sortedValues []cemi.GroupAddr

func knxNewMessage(event knx.GroupEvent) {
	msg := knxMsg{When: time.Now(), Event: event}
	mutex.Lock()
	messages = append(messages, msg)
	if _, ok := values[event.Destination]; !ok {
		// this destination has not been seen yet
		log.Printf("New destination group addr: %v", event.Destination)
		sortedValues = append(sortedValues, event.Destination)
		sort.Slice(sortedValues, func(i, j int) bool { return sortedValues[i] < sortedValues[j] })
	}
	values[event.Destination] = msg
	mutex.Unlock()
	fmt.Println(msg)
	// log.Printf("KNX: %+v", event)
	// b, _ := json.Marshal(event)
	// log.Printf("JSON: %v", string(b))
}

func knxGetMessages(knxrouter string) {
	if !strings.Contains(knxrouter, ":") {
		knxrouter = fmt.Sprintf("%s:%d", knxrouter, KNXDefaultPort)
	}

	for {
		log.Println("Stablishing connection to KNX router")

		client, err := knx.NewGroupTunnel(knxrouter, knx.DefaultTunnelConfig)
		if err != nil {
			log.Fatal(err)
		}
		defer client.Close()

		knxChan := client.Inbound()

	innerLoop:
		for {
			select {
			case <-time.After(KNXTimeout * time.Second):
				log.Printf("timeout (%d seconds)", KNXTimeout)
				break innerLoop
			case event, ok := <-knxChan:
				if !ok {
					log.Printf("not ok")
					break innerLoop
				}
				knxNewMessage(event)
			}
		}
		client.Close()
		time.Sleep(time.Second)
	}
}

func webRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ROOT: %s\n", r.URL)
}

func webGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]
	var a, b, c uint8
	if path == "latest" {
		mutex.Lock()
		msg := messages[len(messages)-1]
		mutex.Unlock()
		fmt.Fprintf(w, "Last message: %+v", msg)
	} else if path == "all" {
		mutex.Lock()
		for i := range sortedValues {
			fmt.Fprintf(w, "%+v\n", values[sortedValues[i]])
		}
		mutex.Unlock()
	} else if i, _ := fmt.Sscanf(path, "%d/%d/%d", &a, &b, &c); i == 3 {
		mutex.Lock()
		fmt.Fprintf(w, "Last message to %d/%d/%d: %+v", a, b, c, values[cemi.NewGroupAddr3(a, b, c)])
		mutex.Unlock()
	}
}

func webSet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "SET: %s", r.URL)
}

func main() {
	if err := ReadConfig("knx.cfg"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("devices: %v\n", devices)
	fmt.Printf("addresses: %v\n", addresses)
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	knxrouter := flag.String("knx", "", "address of KNX router")
	flag.Parse()

	go knxGetMessages(*knxrouter)

	http.HandleFunc("/", webRoot)
	http.HandleFunc("/get/", webGet)
	http.HandleFunc("/set/", webSet)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
