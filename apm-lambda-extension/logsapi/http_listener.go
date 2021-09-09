package logsapi

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// LogsApiHttpListener is used to listen to the Logs API using HTTP
type LogsApiHttpListener struct {
	httpServer *http.Server

	logChannel chan string
}

// NewLogsApiHttpListener returns a LogsApiHttpListener with the given log queue
func NewLogsApiHttpListener(lc chan string) (*LogsApiHttpListener, error) {

	return &LogsApiHttpListener{
		httpServer: nil,
		logChannel: lc,
	}, nil
}

func ListenOnAddress() string {
	env_aws_local, ok := os.LookupEnv("AWS_SAM_LOCAL")
	if ok && "true" == env_aws_local {
		return ":" + DefaultHttpListenerPort
	}

	return "sandbox:" + DefaultHttpListenerPort
}

// Start initiates the server in a goroutine where the logs will be sent
func (s *LogsApiHttpListener) Start() (bool, error) {
	address := ListenOnAddress()
	s.httpServer = &http.Server{Addr: address}
	http.HandleFunc("/", s.http_handler)
	go func() {
		println("Serving logs api requests on %s", address)
		err := s.httpServer.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Printf("Unexpected stop on Http Server: %v", err)
			s.Shutdown()
		} else {
			log.Printf("Http Server closed %v", err)
		}
	}()
	return true, nil
}

// http_handler handles the requests coming from the Logs API.
// Everytime Logs API sends logs, this function will read the logs from the response body
// and put them into a synchronous queue to be read by the main goroutine.
// Logging or printing besides the error cases below is not recommended if you have subscribed to receive extension logs.
// Otherwise, logging here will cause Logs API to send new logs for the printed lines which will create an infinite loop.
func (h *LogsApiHttpListener) http_handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		println("Error reading body: %+v", err)
		return
	}

	// Send the log string to the channel
	h.logChannel <- string(body)
}

func (s *LogsApiHttpListener) Shutdown() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := s.httpServer.Shutdown(ctx)
		if err != nil {
			println("Failed to shutdown http server gracefully %s", err)
		} else {
			s.httpServer = nil
		}
	}
}
