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
		log.Println("Receiving bytes from request")
		handler.data <- bodyBytes
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
