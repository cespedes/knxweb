package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/vapourismo/knx-go/knx/cemi"
)

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

func (s *Server) WebServer() {
	http.HandleFunc("/", s.webRoot)
	http.HandleFunc("/get/", s.webGet)
	http.HandleFunc("/set/", s.webSet)
	// TODO: specify webport in config file
	webport := 8001
	log.Printf("Starting web server on port %d...", webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", webport), nil))
}
