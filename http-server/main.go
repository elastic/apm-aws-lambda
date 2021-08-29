package main

import "fmt"
import "net/http"
import "io/ioutil"
// import "log"
import "time"
import "bytes"
import "compress/gzip"
import "os"

type serverHandler struct{
	data chan []byte
}

func(handler *serverHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler.data <- []byte("hello handler")
	// serverHandler.data <- []byte("hello handler")
	// fmt.Println("hello handler")
	// var bodyBytes []byte
	// if r.Body != nil {
	// 		bodyBytes, _ = ioutil.ReadAll(r.Body)
	// }
	// w.Write([]byte("Hello World"))
	// w.Write(bodyBytes)
	if "/intake/v2/events" == r.URL.Path {
		var bodyBytes []byte
		if r.Body != nil {
				bodyBytes, _ = ioutil.ReadAll(r.Body)
		}


		// decompress
		reader := bytes.NewReader([]byte(bodyBytes))
		gzreader, err1 := gzip.NewReader(reader);
		if(err1 != nil){
				fmt.Println("err1")
				fmt.Println(err1); // Maybe panic here, depends on your error handling.
				os.Exit(1)
		}

		output, err2 := ioutil.ReadAll(gzreader);
		if(err2 != nil){
				fmt.Println("err2")
				fmt.Println(err2);
				os.Exit(1)
		}
		// end decompress
		fmt.Println("These are my bodyBytes")
		fmt.Printf("These are my bodyBytes: %v\n", string(bodyBytes))
		fmt.Printf("These are my decompressed bytes: %v\n", string(output))
		fmt.Printf("%v", r.Header)
		// w.Write([]byte("Hello World"))
		// w.Write(bodyBytes)

		w.Write([]byte("this is a test"))
	} else {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(r.URL.Path))
	}
}

func main() {
		dataChannel := make(chan []byte)
		var handler = serverHandler{data: dataChannel}
		s := &http.Server{
			Addr:           ":8080",
			Handler:        &handler,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		}
		go s.ListenAndServe()
		// log.Fatal()

    fmt.Println("hello world")
		fmt.Println(handler.data)
		for (true) {
			select {
				case agentBytes := <-handler.data:
					fmt.Println("Reading from Channel")
					fmt.Println(agentBytes)
			}
		}
}
