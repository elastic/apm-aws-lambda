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
		case agentData := <-dataChannel:
			log.Println("Received bytes from data channel")
			err := PostToApmServer(agentData, config)
			if err != nil {
				log.Printf("Error sending to APM server: %v", err)
			}
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
