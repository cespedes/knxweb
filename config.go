package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vapourismo/knx-go/knx/cemi"
	"github.com/vapourismo/knx-go/knx/dpt"
)

/* Syntax of KNXweb config file:

gateway 192.168.1.11
	...

device 1.1.10 myroom.i/o
	...

address 2/5/7 1.009 myroom.door
	...
*/
type addrNameType struct {
	Name string
	Type dpt.DatapointValue
}

type Config struct {
	Gateways  []string
	Devices   map[cemi.IndividualAddr]string
	Addresses map[cemi.GroupAddr]addrNameType
}

type UnknownDPT []byte

func (d UnknownDPT) Pack() []byte {
	return []byte(d)
}

func (d *UnknownDPT) Unpack(data []byte) error {
	tmp := make([]byte, len(data))
	copy(tmp, data)
	*d = UnknownDPT(tmp)
	return nil
}

func (d UnknownDPT) String() string {
	return fmt.Sprintf("%v", []byte(d))
}

func (d UnknownDPT) Unit() string {
	return ""
}

func ReadConfig(filename string) (*Config, error) {
	var c Config
	c.Devices = make(map[cemi.IndividualAddr]string)
	c.Addresses = make(map[cemi.GroupAddr]addrNameType)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	s := bufio.NewScanner(f)
	lineNum := 0
	for s.Scan() {
		lineNum++
		line := strings.TrimSpace(s.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			// strip comments
			line = strings.TrimSpace(line[0:i])
		}
		if len(line) == 0 {
			// empty line
			continue
		}
		tokens := strings.Fields(line)
		switch tokens[0] {
		case "gateway":
			if len(tokens) != 2 {
				return nil, fmt.Errorf("Syntax error in %s line %d\n", filename, lineNum)
			}
			c.Gateways = append(c.Gateways, tokens[1])
		case "device":
			if len(tokens) != 3 {
				return nil, fmt.Errorf("Syntax error in %s line %d\n", filename, lineNum)
			}
			// fmt.Printf("line %d: new device: %v\n", lineNum, tokens)
			addr, err := cemi.NewIndividualAddrString(tokens[1])
			if err != nil {
				return nil, fmt.Errorf("Error in %s line %d: %w", filename, lineNum, err)
			}
			c.Devices[addr] = tokens[2]
		case "address":
			if len(tokens) != 4 {
				return nil, fmt.Errorf("Syntax error in %s line %d\n", filename, lineNum)
			}
			// fmt.Printf("line %d: new address: %v\n", lineNum, tokens)
			addr, err := cemi.NewGroupAddrString(tokens[1])
			if err != nil {
				return nil, fmt.Errorf("Error in %s line %d: %w", filename, lineNum, err)
			}
			dp, ok := dpt.Produce(tokens[2])
			if !ok {
				fmt.Printf("Warning: %s line %d: unknown type %v\n", filename, lineNum, tokens[2])
				dp = new(UnknownDPT)
			}
			c.Addresses[addr] = addrNameType{Name: tokens[3], Type: dp}
		default:
			return nil, fmt.Errorf("Syntax error in %s line %d: unrecognized token %s\n", filename, lineNum, tokens[0])
		}
	}
	return &c, nil
}
