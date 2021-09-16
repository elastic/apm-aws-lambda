package extension

import (
	"os"
	"testing"
)

func TestNewPersonPositiveAge(t *testing.T) {
	os.Setenv("ELASTIC_APM_SERVER_URL", "foo.example.com")
	os.Setenv("ELASTIC_APM_SECRET_TOKEN", "bar")

	config := ProcessEnv()
	t.Logf("%v", config)

	if config.apmServerEndpoint != "foo.example.com/intake/v2/events" {
		t.Log("Endpoint not set correctly")
		t.Fail()
	}

	if config.apmServerSecretToken != "bar" {
		t.Log("Secret Token not set correctly")
		t.Fail()
	}

	if config.dataReceiverServerPort != ":8200" {
		t.Log("Default port not set correctly")
		t.Fail()
	}

	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("Default timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_SERVER_PORT", ":8201")
	config = ProcessEnv()
	if config.dataReceiverServerPort != ":8201" {
		t.Log("Env port not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "10")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 10 {
		t.Log("Timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS", "foo")
	config = ProcessEnv()
	if config.dataReceiverTimeoutSeconds != 15 {
		t.Log("Timeout not set correctly")
		t.Fail()
	}

	os.Setenv("ELASTIC_APM_API_KEY", "foo")
	config = ProcessEnv()
	if config.apmServerApiKey != "foo" {
		t.Log("API Key not set correctly")
		t.Fail()
	}
}
