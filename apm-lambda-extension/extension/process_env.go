package extension

import (
	"log"
	"os"
	"strconv"
)

type extensionConfig struct {
	apmServerEndpoint          string
	apmServerSecretToken       string
	apmServerApiKey            string
	dataReceiverServerPort     string
	dataReceiverTimeoutSeconds int
}

func getIntFromEnv(name string) (int, error) {
	strValue := os.Getenv(name)
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return -1, err
	}
	return value, nil
}

// pull env into globals
func ProcessEnv() *extensionConfig {
	endpointUri := "/intake/v2/events"
	dataReceiverTimeoutSeconds, err := getIntFromEnv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS")
	if err != nil {
		log.Printf("Could not read ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS, defaulting to 15: %v\n", err)
		dataReceiverTimeoutSeconds = 15
	}

	config := &extensionConfig{
		apmServerEndpoint:          os.Getenv("ELASTIC_APM_LAMBDA_APM_SERVER") + endpointUri,
		apmServerSecretToken:       os.Getenv("ELASTIC_APM_SECRET_TOKEN"),
		apmServerApiKey:            os.Getenv("ELASTIC_APM_API_KEY"),
		dataReceiverServerPort:     os.Getenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT"),
		dataReceiverTimeoutSeconds: dataReceiverTimeoutSeconds,
	}

	if config.dataReceiverServerPort == "" {
		config.dataReceiverServerPort = ":8200"
	}
	if endpointUri == config.apmServerEndpoint {
		log.Fatalln("please set ELASTIC_APM_LAMBDA_APM_SERVER, exiting")
	}
	if config.apmServerSecretToken == "" && config.apmServerApiKey == "" {
		log.Fatalln("please set ELASTIC_APM_SECRET_TOKEN or ELASTIC_APM_API_KEY, exiting")
	}

	return config
}
