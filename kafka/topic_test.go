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

			// Verify the original topic config was modified (removed cleanup.policy for MSK serverless)
			if tt.config.isAWSMSKServerless() && originalConfigLen > len(tt.topic.Config) {
				if _, stillHasCleanupPolicy := tt.topic.Config["cleanup.policy"]; stillHasCleanupPolicy {
					t.Error("Expected cleanup.policy to be removed from original topic config for AWS MSK Serverless")
				}
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
