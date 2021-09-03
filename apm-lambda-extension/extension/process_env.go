package extension

import (
	"log"
	"os"
)

type extensionConfig struct {
	apmServerEndpoint      string
	apmServerSecretToken   string
	dataReceiverServerPort string
}

// pull env into globals
func ProcessEnv() *extensionConfig {
	endpointUri := "/intake/v2/events"
	config := &extensionConfig{
		apmServerEndpoint:      os.Getenv("ELASTIC_APM_SERVER_URL") + endpointUri,
		apmServerSecretToken:   os.Getenv("ELASTIC_APM_SECRET_TOKEN"),
		dataReceiverServerPort: os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"),
	}

	if "" == config.dataReceiverServerPort {
		config.dataReceiverServerPort = ":8200"
	}
	if endpointUri == config.apmServerEndpoint {
		log.Fatalln("please set ELASTIC_APM_SERVER_URL, exiting")
	}
	if "" == config.apmServerSecretToken {
		log.Fatalln("please set ELASTIC_APM_SECRET_TOKEN, exiting")
	}

	return config
}
