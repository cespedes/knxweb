package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/vapourismo/knx-go/knx"
	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
)

func (s *Server) webRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ROOT: %s\n", r.URL)
}

func (s *Server) getAddrs(str string) []cemi.GroupAddr {
	var result []cemi.GroupAddr

	if addr, err := cemi.NewGroupAddrString(str); err != nil {
		return append(result, addr)
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
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}

		s.Mutex.Lock()
		for _, addr := range addrs {
			if msg, ok := s.Values[addr]; ok {
				if nt, ok := config.Addresses[addr]; ok {
					dp, ok := dpt.Produce(nt.DPT)
					if !ok {
						fmt.Printf("Warning: unknown type %v in config file\n", nt.DPT)
						dp = new(UnknownDPT)
					}
					if err := dp.Unpack(msg.Event.Data); err != nil {
						fmt.Fprintf(w, "Error parsing %v for %v\n", msg.Event.Data, msg.Event.Destination)
					} else {
						b, _ := json.Marshal(dp)
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
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
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
	path := r.URL.Path[5:]
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "400 Bad Request", http.StatusBadRequest)
		return
	}
	groupName := strings.Join(parts[0:len(parts)-1], "/")
	value := parts[len(parts)-1]

	var groupAddr cemi.GroupAddr
	var DPT string
	for key, val := range config.Addresses {
		if groupName == val.Name {
			groupAddr = key
			DPT = val.DPT
			break
		}
	}

	s.Mutex.Lock()
	msg, ok := s.Values[groupAddr]
	s.Mutex.Unlock()
	if !ok || msg.Where == "" {
		http.Error(w, "406 Not Acceptable", http.StatusNotAcceptable)
		return
	}
	dp, ok := dpt.Produce(DPT)
	if !ok {
		fmt.Printf("Warning: unknown type %v in config file\n", DPT)
		http.Error(w, "406 Not Acceptable", http.StatusNotAcceptable)
		return
	}
	err := SetDPTFromString(dp, value)
	if err != nil {
		http.Error(w, fmt.Sprintf("400 Bad Request: %s", err.Error()), http.StatusBadRequest)
		return
	}
	s.Mutex.Lock()
	client, ok := s.Conns[msg.Where]
	s.Mutex.Unlock()
	if !ok {
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		return
	}
	if s.Debug {
		log.Printf("client = %v", client)
		log.Printf("Writing to %s: %v=%v", msg.Where, groupAddr, dp)
	}
	event := knx.GroupEvent{
		Command:     knx.GroupWrite,
		Destination: groupAddr,
		Data:        dp.Pack(),
	}
	err = client.Send(event)
	if err != nil {
		http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	s.knxNewMessage(msg.Where, event)

	// TODO: will write msg.Where:groupAddr=dp
	// SetDPTFromString(d dpt.DatapointValue, value string) error {
	fmt.Fprintf(w, "SET: %v=%v\n", groupAddr, dp)
}

func (s *Server) WebServer() {
	// URLs:
	// /get/<group-name>       <- get value of last write to <group-name>
	// /set/<group-name>/value <- write value to <group-name> in the network
	http.HandleFunc("/", s.webRoot)
	http.HandleFunc("/get/", s.webGet)
	http.HandleFunc("/set/", s.webSet)
	// TODO: specify webport in config file
	webport := 8001
	log.Printf("Starting web server on port %d...", webport)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", webport), nil))
}
