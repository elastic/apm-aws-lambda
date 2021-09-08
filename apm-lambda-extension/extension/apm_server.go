package extension

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
)

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
// func PostToApmServer(postBody []byte, apmServerEndpoint string, apmServerSecretToken string) {
func PostToApmServer(postBody []byte, config *extensionConfig) {
	var compressedBytes bytes.Buffer
	w := gzip.NewWriter(&compressedBytes)
	w.Write(postBody)
	w.Write([]byte{10})
	w.Close()

	client := &http.Client{}

	req, err := http.NewRequest("POST", config.apmServerEndpoint, bytes.NewReader(compressedBytes.Bytes()))
	if err != nil {
		log.Fatalf("An Error Occured calling NewRequest %v", err)
	}
	req.Header.Add("Content-Type", "application/x-ndjson")
	req.Header.Add("Content-Encoding", "gzip")
	req.Header.Add("Authorization", "Bearer "+config.apmServerSecretToken)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("An Error Occured calling client.Do %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	log.Println("reading response body")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("there was an error reading the resp body")
		log.Fatalln(err)
	}

	sb := string(body)
	log.Printf("Response Headers: %v\n", resp.Header)
	log.Printf("Response Body: %v\n", sb)
}
