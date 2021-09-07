package extension

import (
	"fmt"
	"net/http"
)

func handleIntakeV2Events(handler *serverHandler, w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := getDecompressedBytesFromRequest(r)
	if nil != err {
		fmt.Println("could not get bytes from body")
	} else {
		handler.data <- bodyBytes
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
