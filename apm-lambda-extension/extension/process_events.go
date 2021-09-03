package extension

import (
	"context"
	"encoding/json"
	"log"
)

func processShutdown() {
	log.Println("Received SHUTDOWN event")
	log.Println("Exiting")
}

func processNonShutdownEvent(dataChannel chan []byte, config *extensionConfig) {
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
	}
}

func ProcessEvents(
	ctx context.Context,
	dataChannel chan []byte,
	extensionClient *Client,
	config *extensionConfig,
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// call Next method of extension API.  This long polling HTTP method
			// will block until there's an invocation of the function
			log.Println("Waiting for event...")
			res, err := extensionClient.NextEvent(ctx)
			if err != nil {
				log.Printf("Error: %v\n")
				log.Println("Exiting")
				return
			}
			log.Printf("Received event: %v\n", PrettyPrint(res))

			// A shutdown event indicates the execution enviornment is shutting down.
			// This is usually due to inactivity.
			if res.EventType == Shutdown {
				processShutdown()
				return
			}

			processNonShutdownEvent(dataChannel, config)
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
