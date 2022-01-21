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
)

const (
	KNXDefaultPort = 3671
	KNXTimeout     = 60 // no messages in several seconds: probable error in connection
)

var config *Config

type Server struct {
	Debug bool

	Mutex        sync.Mutex
	Messages     []knxMsg
	Values       map[cemi.GroupAddr]knxMsg
	SortedValues []cemi.GroupAddr
}

func (k knxMsg) String() string {
	str := k.When.Format("2006-01-02 15:04:05")
	switch k.Event.Command {
	case knx.GroupRead:
		str += " read:"
	case knx.GroupResponse:
		str += " response:"
	case knx.GroupWrite:
		str += " write:"
	default:
		str += " ???:"
	}
	str += " " + k.Event.Source.String() + " " + k.Event.Destination.String() + "=" + fmt.Sprint(k.Event.Data)
	if dev, ok := config.Devices[k.Event.Source]; ok {
		str += " " + dev
	}
	if nt, ok := config.Addresses[k.Event.Destination]; ok {
		t := nt.Type
		if err := t.Unpack(k.Event.Data); err != nil {
			fmt.Printf("Network: Error parsing %v for %v\n", k.Event.Data, k.Event.Destination)
		} else {
			str += " " + nt.Name + "=" + fmt.Sprint(t)
		}
	}
	return str
}

type knxMsg struct {
	When  time.Time
	Event knx.GroupEvent
}

func (s *Server) knxNewMessage(event knx.GroupEvent) {
	msg := knxMsg{When: time.Now(), Event: event}
	Log(msg)
	s.Mutex.Lock()
	s.Messages = append(s.Messages, msg)
	if _, ok := s.Values[event.Destination]; !ok {
		// this destination has not been seen yet
		if s.Debug {
			log.Printf("New destination group addr: %v", event.Destination)
		}
		s.SortedValues = append(s.SortedValues, event.Destination)
		sort.Slice(s.SortedValues, func(i, j int) bool { return s.SortedValues[i] < s.SortedValues[j] })
	}
	s.Values[event.Destination] = msg
	s.Mutex.Unlock()
	fmt.Println(msg)
	// log.Printf("KNX: %+v", event)
	// b, _ := json.Marshal(event)
	// log.Printf("JSON: %v", string(b))
}

func (s *Server) knxGetMessages() {
	for i, gw := range config.Gateways {
		if !strings.Contains(gw, ":") {
			config.Gateways[i] = fmt.Sprintf("%s:%d", gw, KNXDefaultPort)
		}
	}
	if s.Debug {
		fmt.Printf("gateways: %v\n", config.Gateways)
	}

	for _, gw := range config.Gateways {
		go func(gw string) {
			for {
				log.Printf("Stablishing connection to KNX gateway %s...\n", gw)

				client, err := knx.NewGroupTunnel(gw, knx.DefaultTunnelConfig)
				if err != nil {
					log.Printf("knx.NewGroupTunnel (%s): %s", gw, err.Error())
					log.Printf("Sleeping %d seconds...", KNXTimeout)
					time.Sleep(KNXTimeout * time.Second)
					continue
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
							log.Printf("Error reading from KNX channel")
							break innerLoop
						}
						s.knxNewMessage(event)
					}
				}
				client.Close()
				time.Sleep(time.Second)
			}
		}(gw)
	}
}

func (s *Server) webRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ROOT: %s\n", r.URL)
}

func (s *Server) getAddrs(str string) []cemi.GroupAddr {
	var a, b, c uint8
	var result []cemi.GroupAddr
	i, _ := fmt.Sscanf(str, "%d/%d/%d", &a, &b, &c)
	if i == 3 {
		return append(result, cemi.NewGroupAddr3(a, b, c))
	}
	for key, val := range config.Addresses {
		if str == val.Name {
			return append(result, key)
		}
		if strings.HasPrefix(val.Name, str+"/") {
			result = append(result, key)
		}
	}
	return result
}

func (s *Server) webGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]
	if path == "latest" {
		s.Mutex.Lock()
		if len(s.Messages) == 0 {
			s.Mutex.Unlock()
			return
		}
		msg := s.Messages[len(s.Messages)-1]
		s.Mutex.Unlock()
		fmt.Fprintf(w, "%+v\n", msg)
	} else if path == "all" {
		s.Mutex.Lock()
		for i := range s.SortedValues {
			fmt.Fprintf(w, "%+v\n", s.Values[s.SortedValues[i]])
		}
		s.Mutex.Unlock()
	} else if strings.HasPrefix(path, "all/") {
		addrs := s.getAddrs(path[4:])
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}
		s.Mutex.Lock()
		for _, m := range s.Messages {
			for _, a := range addrs {
				if m.Event.Destination == a {
					fmt.Fprintf(w, "%+v\n", m)
				}
			}
		}
		s.Mutex.Unlock()
	} else if strings.HasPrefix(path, "raw/") {
		addrs := s.getAddrs(path[4:])
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}

		s.Mutex.Lock()
		for _, addr := range addrs {
			if msg, ok := s.Values[addr]; ok {
				if nt, ok := config.Addresses[addr]; ok {
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
		s.Mutex.Unlock()
	} else {
		addrs := s.getAddrs(path)
		if len(addrs) == 0 {
			http.Error(w, "404 Not Found", http.StatusBadRequest)
			return
		}
		s.Mutex.Lock()
		for _, addr := range addrs {
			if msg, ok := s.Values[addr]; ok {
				fmt.Fprintf(w, "%+v\n", msg)
			}
		}
		s.Mutex.Unlock()
	}
}

func (s *Server) webSet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "SET: %s", r.URL)
}

func main() {
	var s Server
	s.Values = make(map[cemi.GroupAddr]knxMsg)
	debug := flag.Bool("debug", false, "debugging info")
	webport := flag.Int("port", 8001, "port to listen for incoming connections")
	logdir := flag.String("logdir", "", "directory where logs are stored")
	flag.Parse()
	if *debug {
		s.Debug = true
	}
	if *logdir != "" {
		logDir = *logdir
		fmt.Printf("logdir = %s\n", logDir)
	}

	var err error
	config, err = ReadConfig("knx.cfg")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(config.Gateways) == 0 {
		log.Fatal("No KNX gateway specified.  Please use \"gateway xx.xx.xx.xx\" in config file.")
	}
	if *debug {
		fmt.Printf("gateways: %v\n", config.Gateways)
		fmt.Printf("devices: %v\n", config.Devices)
		fmt.Printf("addresses: %v\n", config.Addresses)
	}

	func() {
		// TODO: specify file location in config file
		file, err := os.Open("status.json")
		if err != nil {
			log.Println(err)
			return
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&s.Values)
		if err != nil {
			log.Println(err)
			return
		}
		for key := range s.Values {
			s.SortedValues = append(s.SortedValues, key)
		}
		sort.Slice(s.SortedValues, func(i, j int) bool { return s.SortedValues[i] < s.SortedValues[j] })

	}()

	go s.knxGetMessages()
	go func() {
		for {
			time.Sleep(30 * time.Second)
			// TODO: create file atomically (race!)
			// TODO: specify file location in config file
			file, err := os.Create("status.json")
			if err != nil {
				log.Println(err)
				return
			}
			encoder := json.NewEncoder(file)
			err = encoder.Encode(s.Values)
			file.Close()
		}
	}()

	http.HandleFunc("/", s.webRoot)
	http.HandleFunc("/get/", s.webGet)
	http.HandleFunc("/set/", s.webSet)
	log.Printf("Starting web server on port %d...", *webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *webport), nil))
}
