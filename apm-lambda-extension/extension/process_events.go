package extension

import (
	"encoding/json"
	"log"
)

func ProcessShutdown() {
	log.Println("Received SHUTDOWN event")
	log.Println("Exiting")
}

func FlushAPMData(dataChannel chan []byte, config *extensionConfig) {
	log.Println("Checking for agent data")
	for {
		select {
		case agentBytes := <-dataChannel:
			log.Println("Received bytes from data channel")
			PostToApmServer(agentBytes, config)
		default:
			log.Println("No more agent data")
			return
		}
	}
}

func PrettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
