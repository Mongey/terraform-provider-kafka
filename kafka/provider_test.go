package kafka

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var testProvider, _ = overrideProvider()
var testBootstrapServers []string = bootstrapServersFromEnv()

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
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
	if err := client.init(); err != nil {
		t.Fatalf("Client could not be initialized %v", err)
	}
}

func overrideProviderFactory() map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"kafka": func() (*schema.Provider, error) {
			return overrideProvider()
		},
	}
}

func overrideProvider() (*schema.Provider, error) {
	log.Println("[INFO] Setting up override for a provider")
	provider := Provider()

	rc, err := accTestProviderConfig()
	if err != nil {
		return nil, err
	}
	diags := provider.Configure(context.Background(), rc)
	if diags.HasError() {
		log.Printf("[ERROR] Could not configure provider %v", diags)
		return nil, fmt.Errorf("Could not configure provider")
	}

	return provider, nil
}

func accTestProviderConfig() (*terraform.ResourceConfig, error) {
	bootstrapServers := bootstrapServersFromEnv()
	bs := make([]interface{}, len(bootstrapServers))

	for i, s := range bootstrapServers {
		bs[i] = s
	}

	raw := map[string]interface{}{
		"bootstrap_servers": bs,
		"kafka_version":     "3.8.0",
	}

	return terraform.NewResourceConfigRaw(raw), nil
}

func bootstrapServersFromEnv() []string {
	fromEnv := strings.Split(os.Getenv("KAFKA_BOOTSTRAP_SERVERS"), ",")
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
