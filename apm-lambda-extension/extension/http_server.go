package extension

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
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

func getDecompressedBytesFromRequest(r *http.Request) ([]byte, error) {
	var rawBytes []byte
	if r.Body != nil {
		rawBytes, _ = ioutil.ReadAll(r.Body)
	}

	// decompress
	var bodyBytes []byte
	var err2 error
	if contains(r.Header["Content-Encoding"], "gzip") {
		reader := bytes.NewReader([]byte(rawBytes))
		gzreader, err1 := gzip.NewReader(reader)
		if err1 != nil {
			fmt.Println("could not create gzip.NewReader")
			return nil, err1
		}

		bodyBytes, err2 = ioutil.ReadAll(gzreader)
		if err2 != nil {
			fmt.Println("could not create ioutil.ReadAll")
			return nil, err2
		}
		// end decompress
	} else {
		bodyBytes = rawBytes
	}
	return bodyBytes, nil
}

func (handler *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if "/intake/v2/events" == r.URL.Path {
		bodyBytes, err := getDecompressedBytesFromRequest(r)
		if nil != err {
			fmt.Println("could not get bytes from body")
		} else {
			handler.data <- bodyBytes
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404"))
	}
}

func NewHttpServer(config *extensionConfig) chan []byte {
	dataChannel := make(chan []byte)
	var handler = serverHandler{data: dataChannel}
	// todo: use configured port value?
	s := &http.Server{
		Addr:           config.dataReceiverServerPort,
		Handler:        &handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go s.ListenAndServe()
	return handler.data
}
