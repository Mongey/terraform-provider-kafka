package kafka

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestUserScramCredentialAcceptanceChecks(t *testing.T) {
	missing := &sarama.DescribeUserScramCredentialsResponse{Results: []*sarama.DescribeUserScramCredentialsResult{{User: "example", ErrorCode: 91}}} // RESOURCE_NOT_FOUND
	present := &sarama.DescribeUserScramCredentialsResponse{Results: []*sarama.DescribeUserScramCredentialsResult{{User: "example", CredentialInfos: []*sarama.UserScramCredentialsResponseInfo{{Mechanism: sarama.SCRAM_MECHANISM_SHA_256, Iterations: 4096}}}}}
	updated := &sarama.DescribeUserScramCredentialsResponse{Results: []*sarama.DescribeUserScramCredentialsResult{{User: "example", CredentialInfos: []*sarama.UserScramCredentialsResponseInfo{{Mechanism: sarama.SCRAM_MECHANISM_SHA_256, Iterations: 8192}}}}}
	unauthorized := &sarama.DescribeUserScramCredentialsResponse{ErrorCode: sarama.ErrClusterAuthorizationFailed}

	for _, tc := range []struct {
		name       string
		check      func(*terraform.State) error
		iterations string
		responses  []any
		wantErr    string
	}{
		{name: "create", check: testResourceUserScramCredentialCheck_withoutIterations, responses: []any{missing, present}},
		{name: "create write-only", check: testResourceUserScramCredentialCheck_writeOnly, responses: []any{missing, present}},
		{name: "update", check: testResourceUserScramCredentialCheck_withIterations, iterations: "8192", responses: []any{present, updated}},
		{name: "update write-only", check: testResourceUserScramCredentialCheck_writeOnlyWithIterations, iterations: "8192", responses: []any{present, updated}},
		{name: "delete", check: testAccCheckUserScramCredentialDestroy, responses: []any{present, missing}},
		{name: "create authorization", check: testResourceUserScramCredentialCheck_withoutIterations, responses: []any{unauthorized, present}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "create write-only authorization", check: testResourceUserScramCredentialCheck_writeOnly, responses: []any{unauthorized, present}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "delete authorization", check: testAccCheckUserScramCredentialDestroy, responses: []any{unauthorized, missing}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "malformed response", check: testResourceUserScramCredentialCheck_withoutIterations, responses: []any{&sarama.DescribeUserScramCredentialsResponse{}, present}, wantErr: "got 0 results (expected 1)"},
		{
			name: "timeout",
			check: func(*terraform.State) error {
				iterations := int32(4096)
				return retry.RetryContext(context.Background(), 100*time.Millisecond,
					testUserScramCredentialRetry(testProvider.Meta().(*LazyClient), "example", "SCRAM-SHA-256", &iterations))
			},
			responses: []any{missing}, wantErr: "could not be found",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := sarama.NewMockBroker(t, 0)
			defer mock.Close()
			mock.SetHandlerByMap(map[string]sarama.MockResponse{
				"MetadataRequest":                     sarama.NewMockMetadataResponse(t).SetBroker(mock.Addr(), mock.BrokerID()).SetController(mock.BrokerID()),
				"DescribeUserScramCredentialsRequest": sarama.NewMockSequence(tc.responses...),
			})
			cfg := sarama.NewConfig()
			cfg.Version = sarama.V3_8_0_0
			cfg.ApiVersionsRequest = false
			cfg.Metadata.RefreshFrequency = 0
			cfg.Admin.Retry.Max = 0
			client, err := sarama.NewClient([]string{mock.Addr()}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = client.Close() }()
			lazy := &LazyClient{inner: &Client{client: client}}
			lazy.once.Do(func() {})
			originalMeta := testProvider.Meta()
			testProvider.SetMeta(lazy)
			defer testProvider.SetMeta(originalMeta)

			state := &terraform.State{Modules: []*terraform.ModuleState{{Resources: map[string]*terraform.ResourceState{
				"kafka_user_scram_credential.test": {Primary: &terraform.InstanceState{
					ID: "example|SCRAM-SHA-256",
					Attributes: map[string]string{
						"username": "example", "scram_mechanism": "SCRAM-SHA-256",
						"scram_iterations": tc.iterations, "password_wo_version": "v1",
					},
				}},
			}}}}
			err = tc.check(state)
			if tc.wantErr == "" && err != nil {
				t.Fatalf("check failed before metadata converged: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			requests := 0
			for _, exchange := range mock.History() {
				if _, ok := exchange.Request.(*sarama.DescribeUserScramCredentialsRequest); ok {
					requests++
				}
			}
			wantRequests := len(tc.responses)
			if tc.wantErr != "" {
				wantRequests = 1
			}
			if requests != wantRequests {
				t.Fatalf("got %d describe requests, want %d", requests, wantRequests)
			}
		})
	}
}
