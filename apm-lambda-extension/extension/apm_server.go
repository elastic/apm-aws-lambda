package extension

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// todo: can this be a streaming or streaming style call that keeps the
//       connection open across invocations?
// func PostToApmServer(postBody []byte, apmServerEndpoint string, apmServerSecretToken string) {
func PostToApmServer(postBody []byte, config *extensionConfig) error {
	var compressedBytes bytes.Buffer
	w := gzip.NewWriter(&compressedBytes)
	w.Write(postBody)
	w.Write([]byte{10})
	w.Close()

	client := &http.Client{}

	req, err := http.NewRequest("POST", config.apmServerEndpoint, bytes.NewReader(compressedBytes.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to create a new request when posting to APM server: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-ndjson")
	req.Header.Add("Content-Encoding", "gzip")

	if config.apmServerApiKey != "" {
		req.Header.Add("Authorization", "ApiKey "+config.apmServerApiKey)
	} else if config.apmServerSecretToken != "" {
		req.Header.Add("Authorization", "Bearer "+config.apmServerSecretToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post to APM server: %v", err)
	}

	//Read the response body
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read the response body after posting to the APM server")
	}

	log.Printf("APM server response body: %v\n", string(body))
	log.Printf("APM server response status code: %v\n", resp.StatusCode)
	return nil
}
