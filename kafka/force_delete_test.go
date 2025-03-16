package kafka

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Create a package-level variable that can be used in tests

func TestTopicForceDelete(t *testing.T) {
	// Create a test topic
	topic := &Topic{
		Name:              "test-topic",
		Partitions:        1,
		ReplicationFactor: 1,
	}
	
	// Create a mock config with ForceDelete enabled
	config := &Config{
		ForceDelete: true,
	}
	
	// Create a mock client
	client := &LazyClient{
		config: config,
	}
	
	// Create mock ResourceData with the topic
	d := schema.TestResourceDataRaw(t, kafkaTopicResource().Schema, map[string]interface{}{
		"name": topic.Name,
	})
	d.SetId(topic.Name)
	
	// Set our test function
	testMetaToTopicFunc = func(d *schema.ResourceData, meta interface{}) Topic {
		return *topic
	}
	
	// Call the topicDelete function with our test function
	diags := topicDelete(context.Background(), d, client)
	
	// Reset our test function
	testMetaToTopicFunc = nil
	
	// Verify no errors
	if diags != nil && diags.HasError() {
		t.Errorf("Expected no errors, got: %v", diags)
	}
	
	// Verify the ID is cleared
	if d.Id() != "" {
		t.Errorf("Expected ID to be cleared, got: %s", d.Id())
	}
}

func TestACLForceDelete(t *testing.T) {
	// Create a mock config with ForceDelete enabled
	config := &Config{
		ForceDelete: true,
	}
	
	// Create a mock client
	client := &LazyClient{
		config: config,
	}
	
	// Create mock ResourceData with the ACL
	d := schema.TestResourceDataRaw(t, kafkaACLResource().Schema, map[string]interface{}{
		"resource_name":       "test-acl",
		"resource_type":       "Topic",
		"acl_principal":       "User:test",
		"acl_host":            "*",
		"acl_operation":       "Read",
		"acl_permission_type": "Allow",
	})
	d.SetId("test-acl")
	
	// Call the aclDelete function
	diags := aclDelete(context.Background(), d, client)
	
	// Verify no errors
	if diags != nil && diags.HasError() {
		t.Errorf("Expected no errors, got: %v", diags)
	}
}

func TestQuotaForceDelete(t *testing.T) {
	// Create a mock config with ForceDelete enabled
	config := &Config{
		ForceDelete: true,
	}
	
	// Create a mock client
	client := &LazyClient{
		config: config,
	}
	
	// Create mock ResourceData with the quota
	d := schema.TestResourceDataRaw(t, kafkaQuotaResource().Schema, map[string]interface{}{
		"entity_name": "User:test",
		"entity_type": "user",
	})
	d.SetId("User:test|user")
	
	// Call the quotaDelete function
	diags := quotaDelete(context.Background(), d, client)
	
	// Verify no errors
	if diags != nil && diags.HasError() {
		t.Errorf("Expected no errors, got: %v", diags)
	}
}

func TestUserScramCredentialForceDelete(t *testing.T) {
	// Create a mock config with ForceDelete enabled
	config := &Config{
		ForceDelete: true,
	}
	
	// Create a mock client
	client := &LazyClient{
		config: config,
	}
	
	// Create mock ResourceData with the user scram credential
	d := schema.TestResourceDataRaw(t, kafkaUserScramCredentialResource().Schema, map[string]interface{}{
		"username":         "test-user",
		"scram_mechanism":  "SCRAM-SHA-256",
		"password":         "test-password",
		"scram_iterations": 4096,
	})
	d.SetId("test-user|SCRAM-SHA-256")
	
	// Call the userScramCredentialDelete function
	diags := userScramCredentialDelete(context.Background(), d, client)
	
	// Verify no errors
	if diags != nil && diags.HasError() {
		t.Errorf("Expected no errors, got: %v", diags)
	}
} 