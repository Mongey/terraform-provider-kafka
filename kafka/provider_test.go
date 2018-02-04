package kafka

import (
	"testing"

	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

var testProvider *schema.Provider
var testProviders map[string]terraform.ResourceProvider

func TestProvider(t *testing.T) {
	if err := Provider().(*schema.Provider).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func testAccPreCheck(t *testing.T) {
}

func accProvider() map[string]terraform.ResourceProvider {
	provider := Provider().(*schema.Provider)
	brokers := []string{"localhost:9092"}
	raw := map[string]interface{}{
		"brokers": brokers,
	}

	rawConfig, err := config.NewRawConfig(raw)
	if err != nil {
		panic(err)
	}

	err = provider.Configure(terraform.NewResourceConfig(rawConfig))
	if err != nil {
		panic(err)
	}

	return map[string]terraform.ResourceProvider{
		"kafka": provider,
	}
}
