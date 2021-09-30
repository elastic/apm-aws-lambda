package extension

import (
	"log"
	"net/http"
)

func handleIntakeV2Events(handler *serverHandler, w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := getDecompressedBytesFromRequest(r)
	if nil != err {
		log.Println("could not get bytes from body")
	} else {
		log.Println("Adding agent data to buffer to be sent to apm server")
		handler.data <- bodyBytes
	}
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("ok"))
}
