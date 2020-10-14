package kafka

import (
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

var testProvider *schema.Provider
var testBootstrapServers []string = bootstrapServersFromEnv()

func TestProvider(t *testing.T) {
	if err := Provider().(*schema.Provider).InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func testAccPreCheck(t *testing.T) {
	meta := testProvider.Meta()
	if meta == nil {
		t.Fatal("Could not construct client")
	}
	client := meta.(*LazyClient)
	if client == nil {
		t.Fatal("No client")
	}
}

func accProvider() map[string]terraform.ResourceProvider {
	log.Println("[INFO] Setting up override for a provider")
	provider := Provider().(*schema.Provider)

	bootstrapServers := []interface{}{}
	for _, bs := range testBootstrapServers {
		bootstrapServers = append(bootstrapServers, bs)
	}

	raw := map[string]interface{}{
		"bootstrap_servers": bootstrapServers,
	}

	err := provider.Configure(terraform.NewResourceConfigRaw(raw))
	if err != nil {
		log.Printf("[ERROR] Could not configure provider %v", err)
	}

	testProvider = provider

	return map[string]terraform.ResourceProvider{
		"kafka": provider,
	}
}

func bootstrapServersFromEnv() []string {
	fromEnv := strings.Split(os.Getenv("KAFKA_BOOTSTRAP_SERVER"), ",")
	fromEnv = nonEmptyAndTrimmed(fromEnv)

	if len(fromEnv) == 0 {
		fromEnv = []string{"localhost:9092"}
	}

	bootstrapServers := make([]string, 0)
	for _, bs := range fromEnv {
		if bs != "" {
			bootstrapServers = append(bootstrapServers, bs)
		}
	}

	return bootstrapServers
}

func nonEmptyAndTrimmed(bootstrapServers []string) []string {
	wellFormed := make([]string, 0)

	for _, bs := range bootstrapServers {
		trimmed := strings.TrimSpace(bs)
		if trimmed != "" {
			wellFormed = append(wellFormed, trimmed)
		}
	}

	return wellFormed
}
