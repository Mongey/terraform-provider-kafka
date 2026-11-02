package kafka

import (
	"reflect"
	"testing"
)

func TestCopyWithMaskedSensitiveValues(t *testing.T) {
	original := Config{
		ClientCertKey:           "private-key",
		ClientCertKeyPassphrase: "passphrase",
		SASLPassword:            "password",
		SASLAWSExternalId:       "external-id",
		SASLAWSSecretKey:        "secret-key",
		SASLAWSToken:            "session-token",
		SASLAWSAccessKey:        "access-key",
		SASLUsername:            "username",
	}
	want := original
	want.ClientCertKey = "*****"
	want.ClientCertKeyPassphrase = "*****"
	want.SASLPassword = "*****"
	want.SASLAWSExternalId = "*****"
	want.SASLAWSSecretKey = "*****"
	want.SASLAWSToken = "*****"

	got := original.copyWithMaskedSensitiveValues()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("masked config = %+v, want %+v", got, want)
	}
	if original.SASLAWSToken != "session-token" {
		t.Error("masking modified the original config")
	}
}
