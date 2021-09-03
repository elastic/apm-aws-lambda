package extension

import (
	"log"
	"os"
)

type extensionConfig struct {
	apmServerEndpoint    string
	apmServerSecretToken string
}

// pull env into globals
func ProcessEnv() *extensionConfig {
	endpointUri := "/intake/v2/events"
	config := &extensionConfig{
		apmServerEndpoint:    os.Getenv("ELASTIC_APM_SERVER_URL") + endpointUri,
		apmServerSecretToken: os.Getenv("ELASTIC_APM_SECRET_TOKEN"),
	}

	if endpointUri == config.apmServerEndpoint {
		log.Fatalln("please set ELASTIC_APM_SERVER_URL, exiting")
	}
	if "" == config.apmServerSecretToken {
		log.Fatalln("please set ELASTIC_APM_SECRET_TOKEN, exiting")
	}

	return config
}
