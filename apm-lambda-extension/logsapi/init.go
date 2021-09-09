package logsapi

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
)

const DefaultHttpListenerPort = "1234"

// Init initializes the configuration for the Logs API and subscribes to the Logs API for HTTP
func Subscribe(extensionID string, eventTypes []EventType) error {
	extensions_api_address, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	if !ok {
		return errors.New("AWS_LAMBDA_RUNTIME_API is not set")
	}

	logsApiBaseUrl := fmt.Sprintf("http://%s", extensions_api_address)

	logsApiClient, err := NewClient(logsApiBaseUrl)
	if err != nil {
		return err
	}

	bufferingCfg := BufferingCfg{
		MaxItems:  10000,
		MaxBytes:  262144,
		TimeoutMS: 1000,
	}
	if err != nil {
		return err
	}
	destination := Destination{
		Protocol:   HttpProto,
		URI:        URI(fmt.Sprintf("http://sandbox:%s", DefaultHttpListenerPort)),
		HttpMethod: HttpPost,
		Encoding:   JSON,
	}

	_, err = logsApiClient.Subscribe(eventTypes, bufferingCfg, destination, extensionID)
	return err
}
