package kafka

import (
	"context"
	"encoding/json"
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
	return overrideProviderFull(map[string]interface{}{})
}

func overrideProviderFull(configOverrides map[string]interface{}) (*schema.Provider, error) {

	log.Println("[INFO] Setting up override for a provider")
	provider := Provider()

	rc, err := accTestProviderConfig(configOverrides)
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

func accTestProviderConfig(overrides map[string]interface{}) (*terraform.ResourceConfig, error) {
	bootstrapServers := bootstrapServersFromEnv()
	bs := make([]interface{}, len(bootstrapServers))

	for i, s := range bootstrapServers {
		bs[i] = s
	}

	raw := map[string]interface{}{
		"bootstrap_servers": bs,
		"kafka_version":     "3.8.0",
	}

	jsonOverrides, err := json.Marshal(overrides)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal overrides: %v", err))
	}

	var newOverrides map[string]interface{}
	err = json.Unmarshal(jsonOverrides, &newOverrides)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal JSON: %v", err))
	}

	// Apply overrides
	for k, v := range newOverrides {
		if _, exists := raw[k]; !exists {
			raw[k] = v
		}
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

func ProviderChecker(t *testing.T, configOverrides map[string]interface{}) *Config {
	provider, err := overrideProviderFull(configOverrides)
	if err != nil {
		t.Fatalf("could not configure provider: %v", err)
	}

	meta := provider.Meta()
	if meta == nil {
		t.Fatal("meta is nil")
	}

	cfg := meta.(*LazyClient).Config
	if cfg == nil {
		t.Fatal("expected non-nil Config")
	}

	return cfg
}

func Test_ProviderBaseConfig(t *testing.T) {
	cfg := ProviderChecker(t, map[string]interface{}{})

	if len(cfg.FailOn) != 0 {
		t.Fatalf("expected 0 fail_on conditions, got %d", len(cfg.FailOn))
	}
	if Contains(cfg.FailOn, "partition_lower") {
		t.Fatalf("fail_on contains 'partition_lower' but shouldn't: %+v", cfg.FailOn)
	}
}

func Test_ProviderFailOnConfig(t *testing.T) {
	var failOn []string
	failOn = append(failOn, "partition_lower")
	extraConfig := map[string]interface{}{
		"fail_on": failOn,
	}
	cfg := ProviderChecker(t, extraConfig)

	if len(cfg.FailOn) == 0 {
		t.Fatalf("expected 1 fail_on conditions, got %d", len(cfg.FailOn))
	}
	if !Contains(cfg.FailOn, "partition_lower") {
		t.Fatalf("fail_on missing 'partition_lower' but shouldn't: %+v", cfg.FailOn)
	}
}

func Test_ProviderFailOnEmptyConfig(t *testing.T) {
	cfg := ProviderChecker(t, map[string]interface{}{
		"fail_on": []string{},
	})

	if len(cfg.FailOn) != 0 {
		t.Fatalf("expected 0 fail_on conditions, got %d", len(cfg.FailOn))
	}
}
