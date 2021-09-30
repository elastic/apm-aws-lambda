package extension

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type serverHandler struct {
	data chan []byte
}

func contains(haystack []string, needle string) bool {
	for _, value := range haystack {
		if value == needle {
			return true
		}
	}
	return false
}

func getDecompressedBytesFromRequest(req *http.Request) ([]byte, error) {
	var rawBytes []byte
	if req.Body != nil {
		rawBytes, _ = ioutil.ReadAll(req.Body)
	}

	switch req.Header.Get("Content-Encoding") {
	case "deflate":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := zlib.NewReader(reader)
		if err != nil {
			log.Printf("Error: could not create zlib.NewReader. %v", err)
			return nil, err
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			fmt.Println("Could not read from zlib reader using ioutil.ReadAll")
			return nil, err
		}
		return bodyBytes, nil
	case "gzip":
		reader := bytes.NewReader([]byte(rawBytes))
		zlibreader, err := gzip.NewReader(reader)
		if err != nil {
			log.Printf("Error: could not create gzip.NewReader. %v", err)
			return nil, err
		}
		bodyBytes, err := ioutil.ReadAll(zlibreader)
		if err != nil {
			fmt.Println("Could not read from gzip reader using ioutil.ReadAll")
			return nil, err
		}
		return bodyBytes, nil
	default:
		return rawBytes, nil
	}
}

func (handler *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if "/intake/v2/events" == r.URL.Path {
		handleIntakeV2Events(handler, w, r)
		return
	}

	// if we have not yet returned, 404
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404"))

}

func NewHttpServer(dataChannel chan []byte, config *extensionConfig) *http.Server {
	var handler = serverHandler{data: dataChannel}
	timeout := time.Duration(config.dataReceiverTimeoutSeconds) * time.Second
	s := &http.Server{
		Addr:           config.dataReceiverServerPort,
		Handler:        &handler,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: 1 << 20,
	}
	go s.ListenAndServe()
	return s
}
