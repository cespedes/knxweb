package main

import (
	"encoding/json"
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
	KNXTimeout     = 60 // no messages in several seconds: probable error in connection
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
		t := nt.Type
		if err := t.Unpack(k.Event.Data); err != nil {
			fmt.Printf("Network: Error parsing %v for %v\n", k.Event.Data, k.Event.Destination)
		} else {
			s += " " + nt.Name + "=" + fmt.Sprint(t)
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
	Log(msg)
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
		log.Printf("Stablishing connection to KNX router %s...\n", knxrouter)

		client, err := knx.NewGroupTunnel(knxrouter, knx.DefaultTunnelConfig)
		if err != nil {
			log.Fatalf("knx.NewGroupTunnel: %s", err.Error())
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

func getAddrs(s string) []cemi.GroupAddr {
	var a, b, c uint8
	var result []cemi.GroupAddr
	i, _ := fmt.Sscanf(s, "%d/%d/%d", &a, &b, &c)
	if i == 3 {
		return append(result, cemi.NewGroupAddr3(a, b, c))
	}
	for key, val := range addresses {
		if s == val.Name {
			return append(result, key)
		}
		if strings.HasPrefix(val.Name, s+"/") {
			result = append(result, key)
		}
	}
	return result
}

func webGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]
	if path == "latest" {
		mutex.Lock()
		if len(messages) == 0 {
			mutex.Unlock()
			return
		}
		msg := messages[len(messages)-1]
		mutex.Unlock()
		fmt.Fprintf(w, "%+v\n", msg)
	} else if path == "all" {
		mutex.Lock()
		for i := range sortedValues {
			fmt.Fprintf(w, "%+v\n", values[sortedValues[i]])
		}
		mutex.Unlock()
	} else if strings.HasPrefix(path, "all/") {
		addrs := getAddrs(path[4:])
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}
		mutex.Lock()
		for _, m := range messages {
			for _, a := range addrs {
				if m.Event.Destination == a {
					fmt.Fprintf(w, "%+v\n", m)
				}
			}
		}
		mutex.Unlock()
	} else if strings.HasPrefix(path, "raw/") {
		addrs := getAddrs(path[4:])
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}

		mutex.Lock()
		for _, addr := range addrs {
			if msg, ok := values[addr]; ok {
				if nt, ok := addresses[addr]; ok {
					t := nt.Type
					if err := t.Unpack(msg.Event.Data); err != nil {
						fmt.Fprintf(w, "Error parsing %v for %v\n", msg.Event.Data, msg.Event.Destination)
					} else {
						b, _ := json.Marshal(t)
						fmt.Fprintf(w, "%s\n", string(b))
					}
				} else {
					fmt.Fprintf(w, "%v\n", msg.Event.Data)
				}
			}
		}
		mutex.Unlock()
	} else {
		addrs := getAddrs(path)
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}
		mutex.Lock()
		for _, addr := range addrs {
			if msg, ok := values[addr]; ok {
				fmt.Fprintf(w, "%+v\n", msg)
			}
		}
		mutex.Unlock()
	}
}

func webSet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "SET: %s", r.URL)
}

func main() {
	debug := flag.Bool("debug", false, "debugging info")
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	knxrouter := flag.String("knx", "", "address of KNX router")
	logdir := flag.String("logdir", "", "directory where logs are stored")
	flag.Parse()
	if *logdir != "" {
		logDir = *logdir
		fmt.Printf("logdir = %s\n", logDir)
	}

	if *knxrouter == "" {
		log.Fatal("No KNX router specified.  Please use option -knx")
	}
	if err := ReadConfig("knx.cfg"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *debug {
		fmt.Printf("devices: %v\n", devices)
		fmt.Printf("addresses: %v\n", addresses)
	}

	go knxGetMessages(*knxrouter)

	http.HandleFunc("/", webRoot)
	http.HandleFunc("/get/", webGet)
	http.HandleFunc("/set/", webSet)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
