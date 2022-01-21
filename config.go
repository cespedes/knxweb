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

device 1.1.10 myroom.i/o
	...

address 2/5/7 1.009 myroom.door
	...
*/

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

func ReadConfig(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
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
		case "device":
			if len(tokens) != 3 {
				return fmt.Errorf("%s line %d: %s\n", filename, lineNum, line)
			}
			// fmt.Printf("line %d: new device: %v\n", lineNum, tokens)
			addr, err := cemi.NewIndividualAddrString(tokens[1])
			if err != nil {
				return fmt.Errorf("%s line %d: %w", filename, lineNum, err)
			}
			devices[addr] = tokens[2]
		case "address":
			if len(tokens) != 4 {
				return fmt.Errorf("Syntax error in line %d: %s\n", lineNum, line)
			}
			// fmt.Printf("line %d: new address: %v\n", lineNum, tokens)
			addr, err := cemi.NewGroupAddrString(tokens[1])
			if err != nil {
				return fmt.Errorf("%s line %d: %w", filename, lineNum, err)
			}
			dp, ok := dpt.Produce(tokens[2])
			if !ok {
				fmt.Printf("Warning: %s line %d: unknown type %v\n", filename, lineNum, tokens[2])
				dp = new(UnknownDPT)
			}
			addresses[addr] = addrNameType{Name: tokens[3], Type: dp}
		default:
			return fmt.Errorf("Syntax error in line %d: %s\n", lineNum, line)
		}
	}
	return nil
}
