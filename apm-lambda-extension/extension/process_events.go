package extension

import (
	"encoding/json"
	"log"
	"time"
)

func ProcessShutdown() {
	log.Println("Received SHUTDOWN event")
	log.Println("Exiting")
}

func ProcessAPMData(dataChannel chan []byte, config *extensionConfig) {
	log.Printf("Reading APM data")
	// Wait for agent to send data to the channel
	//
	// to do: this will hang if the lambda times out or crashes or the agent
	//        fails to send data to the pipe
	select {
	case agentBytes := <-dataChannel:
		log.Printf("received bytes from data channel %v", agentBytes)
		PostToApmServer(
			agentBytes,
			config,
		)
		log.Println("done with post")
	case <-time.After(1 * time.Second):
		log.Println("timed out waiting for APM data")
	}
}

func PrettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
