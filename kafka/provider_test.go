package kafka

import (
	"log"
	"os"
	"strings"
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
	client := testProvider.Meta().(*Client)
	if client == nil {
		t.Fatal("No client")
	}
}

func accProvider() map[string]terraform.ResourceProvider {
	log.Println("[INFO] Setting up override for a provider")
	provider := Provider().(*schema.Provider)
	bs := strings.Split(os.Getenv("KAFKA_BOOTSTRAP_SERVER"), ",")
	bootstrapServers := []string{}

	for _, v := range bs {
		if v != "" {
			bootstrapServers = append(bootstrapServers, v)
		}
	}
	raw := map[string]interface{}{
		"bootstrap_servers": bootstrapServers,
	}

	rawConfig, _ := config.NewRawConfig(raw)
	_ = provider.Configure(terraform.NewResourceConfig(rawConfig))
	testProvider = provider
	return map[string]terraform.ResourceProvider{
		"kafka": provider,
	}
}
