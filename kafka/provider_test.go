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
var testProviders map[string]terraform.ResourceProvider

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
	client := meta.(*Client)
	if client == nil {
		t.Fatal("No client")
	}
}

func accProvider() map[string]terraform.ResourceProvider {
	log.Println("[INFO] Setting up override for a provider")
	provider := Provider().(*schema.Provider)

	bs := strings.Split(os.Getenv("KAFKA_BOOTSTRAP_SERVER"), ",")
	if len(bs) == 0 {
		bs = []string{"localhost:9092"}
	}

	bootstrapServers := []interface{}{}

	for _, v := range bs {
		if v != "" {
			bootstrapServers = append(bootstrapServers, v)
		}
	}

	raw := map[string]interface{}{
		"bootstrap_servers": bootstrapServers,
	}

	err := provider.Configure(terraform.NewResourceConfigRaw(raw))
	if err != nil {
		log.Printf("[ERROR] Could note configure provider %v", err)
	}

	testProvider = provider

	return map[string]terraform.ResourceProvider{
		"kafka": provider,
	}
}
