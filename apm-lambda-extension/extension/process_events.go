package extension

import (
	"context"
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

func PollNextEvent(extensionClient *Client, ctx context.Context) chan *NextEventResponse {
	eventChan := make(chan *NextEventResponse)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				log.Println("Waiting for event...")
				res, err := extensionClient.NextEvent(ctx)
				if err != nil {
					log.Printf("Error polling for next event: %v\n", err)
					// ToDo stop polling here?
				} else {
					eventChan <- res
				}
			}
		}
	}()

	return eventChan
}
