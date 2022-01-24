package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
	KNXTimeout     = 120 // no messages in several seconds: probable error in connection
)

var config *Config

type Server struct {
	Debug bool

	Mutex        sync.Mutex
	Messages     []knxMsg
	Values       map[cemi.GroupAddr]knxMsg
	SortedValues []cemi.GroupAddr

	Conns map[string]knx.GroupTunnel

	logFile     *os.File
	logFileName string
}

type knxMsg struct {
	When  time.Time
	Where string // Gateway where this message came from
	Event knx.GroupEvent
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
		dp, ok := dpt.Produce(nt.DPT)
		if !ok {
			fmt.Printf("Warning: unknown type %v in config file\n", nt.DPT)
			dp = new(UnknownDPT)
		}
		if err := dp.Unpack(k.Event.Data); err != nil {
			fmt.Printf("Network: Error parsing %v for %v\n", k.Event.Data, k.Event.Destination)
		} else {
			str += " " + nt.Name + "=" + fmt.Sprint(dp)
		}
	}
	return str
}

func (s *Server) knxNewMessage(gateway string, event knx.GroupEvent) {
	msg := knxMsg{When: time.Now(), Where: gateway, Event: event}
	s.Log(msg)
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
		if !strings.Contains(gw.Address, ":") {
			config.Gateways[i].Address = fmt.Sprintf("%s:%d", gw.Address, KNXDefaultPort)
		}
	}
	if s.Debug {
		fmt.Printf("gateways: %v\n", config.Gateways)
	}

	s.Conns = make(map[string]knx.GroupTunnel)
	for _, gw := range config.Gateways {
		go func(gwName string) {
			for {
				log.Printf("Stablishing connection to KNX gateway %s...\n", gwName)

				client, err := knx.NewGroupTunnel(gwName, knx.DefaultTunnelConfig)
				if err != nil {
					log.Printf("knx.NewGroupTunnel (%s): %s", gwName, err.Error())
					log.Printf("Sleeping %d seconds...", KNXTimeout)
					time.Sleep(KNXTimeout * time.Second)
					continue
				}
				defer client.Close()
				s.Mutex.Lock()
				s.Conns[gwName] = client
				s.Mutex.Unlock()

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
						s.knxNewMessage(gwName, event)
					}
				}
				s.Mutex.Lock()
				client.Close()
				delete(s.Conns, gwName)
				s.Mutex.Unlock()
				time.Sleep(time.Second)
			}
		}(gw.Address)
	}
}

func main() {
	var s Server
	s.Values = make(map[cemi.GroupAddr]knxMsg)
	debug := flag.Bool("debug", false, "debugging info")
	configFile := flag.String("config", "knx.cfg", "config file")
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
	config, err = ReadConfig(*configFile)
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
		// TODO: compress?
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
			if s.Debug {
				log.Println("Writing status to disk")
			}
			// TODO: create file atomically (race!)
			// TODO: specify file location in config file
			// TODO: compress?
			file, err := os.Create("status.json")
			if err != nil {
				log.Println(err)
				return
			}
			encoder := json.NewEncoder(file)
			s.Mutex.Lock()
			err = encoder.Encode(s.Values)
			s.Mutex.Unlock()
			file.Close()
		}
	}()

	s.WebServer()
}
