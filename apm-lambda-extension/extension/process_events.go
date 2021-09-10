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

func ProcessAPMData(agentBytes []byte, config *extensionConfig) {
	PostToApmServer(agentBytes, config)
}

func FlushAPMData(dataChannel chan []byte, config *extensionConfig) {
	select {
	case agentBytes := <-dataChannel:
		PostToApmServer(agentBytes, config)
	case <-time.After(1 * time.Second):
		log.Println("Time expired waiting for agent bytes. No more bytes will be sent.")
	}
}

func PrettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
