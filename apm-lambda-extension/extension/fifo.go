package extension

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"syscall"
)

var (
	socketPath = "/tmp/elastic-apm-data"
)

func setupNamedSocket() chan []byte {
	err := syscall.Mkfifo(socketPath, 0666)
	if err != nil {
		log.Printf("error creating pipe %v\n", err)
	}
	dataChannel := make(chan []byte)
	go func() {
		for {
			dataChannel <- pollSocketForData()
		}
	}()
	return dataChannel
}

func pollSocketForData() []byte {
	dataPipe, err := os.OpenFile(socketPath, os.O_RDONLY, 0)
	if err != nil {
		log.Fatalln("failed to open pipe", err)
	}

	defer close(dataPipe)

	// When the write side closes, we get an EOF.
	bytes, err := ioutil.ReadAll(dataPipe)
	if err != nil {
		log.Fatalln("failed to read telemetry pipe", err)
	}

	return bytes
}

func close(io io.Closer) {
	err := io.Close()
	if err != nil {
		log.Println(err)
	}
}
