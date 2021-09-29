package logsapi

import (
	"os"
	"testing"
)

func TestListenOnAddressWithENVVariable(t *testing.T) {
	os.Setenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS", "example:3456")

	address := ListenOnAddress()
	t.Logf("%v", address)

	if address != "example:3456" {
		t.Log("Address was not taken from ENV variable correctly")
		t.Fail()
	}
}

func TestListenOnAddressDefault(t *testing.T) {
	os.Unsetenv("ELASTIC_APM_LAMBDA_LOGS_LISTENER_ADDRESS")
	address := ListenOnAddress()
	t.Logf("%v", address)

	if address != "sandbox:1234" {
		t.Log("Default address was not used")
		t.Fail()
	}
}
