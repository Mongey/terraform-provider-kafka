package kafka

import (
	"testing"

	"github.com/IBM/sarama"
)

func TestConfigToResources_RemovesCleanupPolicyForAWSMSKServerless(t *testing.T) {
	tests := []struct {
		name                string
		topic               Topic
		config              *Config
		expectCleanupPolicy bool
	}{
		{
			name: "AWS MSK Serverless - cleanup.policy removed",
			topic: Topic{
				Name: "test-topic",
				Config: map[string]*string{
					"cleanup.policy": stringPtr("delete"),
					"retention.ms":   stringPtr("604800000"),
				},
			},
			config: &Config{
				BootstrapServers: &[]string{"kafka-serverless.us-east-1.amazonaws.com:9092"},
			},
			expectCleanupPolicy: false,
		},
		{
			name: "Non-MSK Serverless - cleanup.policy retained",
			topic: Topic{
				Name: "test-topic",
				Config: map[string]*string{
					"cleanup.policy": stringPtr("delete"),
					"retention.ms":   stringPtr("604800000"),
				},
			},
			config: &Config{
				BootstrapServers: &[]string{"localhost:9092"},
			},
			expectCleanupPolicy: true,
		},
		{
			name: "AWS MSK Serverless - no cleanup.policy to remove",
			topic: Topic{
				Name: "test-topic",
				Config: map[string]*string{
					"retention.ms": stringPtr("604800000"),
				},
			},
			config: &Config{
				BootstrapServers: &[]string{"kafka-serverless.us-east-1.amazonaws.com:9092"},
			},
			expectCleanupPolicy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of the config map to check original isn't modified
			originalConfigLen := len(tt.topic.Config)

			resources := configToResources(tt.topic, tt.config)

			if len(resources) != 1 {
				t.Errorf("Expected 1 resource, got %d", len(resources))
				return
			}

			resource := resources[0]

			// Check resource type and name
			if resource.Type != sarama.TopicResource {
				t.Errorf("Expected TopicResource type, got %v", resource.Type)
			}

			if resource.Name != tt.topic.Name {
				t.Errorf("Expected topic name %s, got %s", tt.topic.Name, resource.Name)
			}

			// Check if cleanup.policy is present or removed as expected
			_, hasCleanupPolicy := resource.ConfigEntries["cleanup.policy"]
			if hasCleanupPolicy != tt.expectCleanupPolicy {
				t.Errorf("Expected cleanup.policy presence to be %v, but was %v", tt.expectCleanupPolicy, hasCleanupPolicy)
			}

			// Verify the original topic config is NOT modified (we should not mutate the caller's map)
			if len(tt.topic.Config) != originalConfigLen {
				t.Error("Original topic config should not be mutated by configToResources")
			}
		})
	}
}

// TestConfigToResources_DoesNotMutateCallerMap verifies that configToResources
// does not mutate the caller's topic.Config map. This is a regression test for
// a bug where the function would delete entries from the original map.
func TestConfigToResources_DoesNotMutateCallerMap(t *testing.T) {
	// Create a topic with cleanup.policy
	originalConfig := map[string]*string{
		"cleanup.policy": stringPtr("delete"),
		"retention.ms":   stringPtr("604800000"),
	}

	topic := Topic{
		Name:   "test-topic",
		Config: originalConfig,
	}

	// Use AWS MSK Serverless config which triggers the cleanup.policy removal
	config := &Config{
		BootstrapServers: &[]string{"kafka-serverless.us-east-1.amazonaws.com:9092"},
	}

	// Verify config has cleanup.policy before the call
	if _, exists := topic.Config["cleanup.policy"]; !exists {
		t.Fatal("Test setup error: cleanup.policy should exist before configToResources call")
	}

	originalLen := len(topic.Config)

	// Call configToResources
	resources := configToResources(topic, config)

	// The returned resources should NOT have cleanup.policy (correct behavior for MSK Serverless)
	if _, exists := resources[0].ConfigEntries["cleanup.policy"]; exists {
		t.Error("Expected cleanup.policy to be removed from returned resources for MSK Serverless")
	}

	// BUG CHECK: The original topic.Config should NOT be modified
	// This test will FAIL before the fix is applied
	if len(topic.Config) != originalLen {
		t.Errorf("Bug: original topic.Config was mutated! Expected %d entries, got %d", originalLen, len(topic.Config))
	}

	if _, exists := topic.Config["cleanup.policy"]; !exists {
		t.Error("Bug: cleanup.policy was deleted from the caller's original map")
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
