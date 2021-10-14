package extension

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
)

/**
 * Starts a mock HTTP server
 *
 * Exposes a single / endpoint.  This endpoint returns
 * a JSON string with two properties
 *
 * data: the value passed via `response`, as JSON
 * seen_headers: the headers seen by the request to /
 *
 * The intended use is to start the extension's HTTP server
 * and confirm that it proxies info requests to this server
 */
func startMockApmServer(response map[string]string, port string) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		seenHeaders := make(map[string]string)
		for name, values := range r.Header {
			for _, value := range values {
				seenHeaders[name] = value
			}
		}

		data := map[string]map[string]string{
			"data":         response,
			"seen_headers": seenHeaders,
		}
		responseString, _ := json.Marshal(data)
		w.Write([]byte(responseString))
	})
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getUrl(url string, headers map[string]string) string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	for name, value := range headers {
		req.Header.Add(name, value)
	}
	resp, _ := client.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}

func startExtension(config *extensionConfig) {
	dataChannel := make(chan []byte, 100)
	NewHttpServer(dataChannel, config)
}

func TestProxy(t *testing.T) {
	var apmServerPort = "8080"

	go startMockApmServer(
		map[string]string{"foo": "bar"},
		apmServerPort,
	)
	go startExtension(&extensionConfig{
		apmServerUrl:               "http://localhost:" + apmServerPort,
		apmServerSecretToken:       "foo",
		apmServerApiKey:            "bar",
		dataReceiverServerPort:     ":8081",
		dataReceiverTimeoutSeconds: 15,
	})

	body := getUrl(
		"http://localhost:8081",
		map[string]string{"Authorization": "test-value"},
	)

	var result map[string]map[string]string
	json.Unmarshal([]byte(body), &result)
	var data map[string]string

	if result["data"]["foo"] != "bar" {
		t.Logf("unexpected value returned: %s", data)
		t.Fail()
	}

	if result["seen_headers"]["Authorization"] != "test-value" {
		t.Logf("did not see Authorization header %s", data)
		t.Fail()
	}
}
