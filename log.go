package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"time"
)

// TODO create 2 different logs: knx-log and general log

var logDir string
var logFile *os.File
var logSize int

const maxLogSize = 16 * 1024 * 1024

func createLogFile() {
	var err error
	t := time.Now()
	filename := t.Format("20060102-150406.knxlog")
	if logDir != "" {
		filename = fmt.Sprintf("%s/%s", logDir, filename)
	}
	logFile, err = os.Create(filename)
	if err != nil {
		panic(err)
	}
	logSize = 0
}

func LogBinary(k knxMsg) {
	var buf [1024]byte
	if logFile == nil {
		createLogFile()
	}
	t := uint32(k.When.Unix())
	binary.BigEndian.PutUint32(buf[:4], t)
	buf[4] = byte(k.Event.Command)
	binary.BigEndian.PutUint16(buf[5:7], uint16(k.Event.Source))
	binary.BigEndian.PutUint16(buf[7:9], uint16(k.Event.Destination))
	buf[9] = byte(len(k.Event.Data))
	copy(buf[10:], k.Event.Data)
	l := 10 + len(k.Event.Data)
	_, err := logFile.Write(buf[0:l])
	if err != nil {
		panic(err)
	}
	logSize += l
	if logSize >= maxLogSize {
		err = logFile.Close()
		if err != nil {
			panic(err)
		}
		logFile = nil
	}
}

func (s *Server) Log(k knxMsg) {
	// LogBinary(k)

	var err error
	filename := path.Join(config.Logdir, time.Now().Format("2006/0102.log"))
	if s.logFileName != filename {
		s.logFile.Close()
		os.MkdirAll(filepath.Dir(filename), 0777)
		s.logFile, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		s.logFileName = filename
	}
	fmt.Fprintf(s.logFile, "%s\n", k)
}
