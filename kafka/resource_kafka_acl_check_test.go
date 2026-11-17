package kafka

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestACLAcceptanceChecks(t *testing.T) {
	missing := &sarama.DescribeAclsResponse{Version: 1}
	present := &sarama.DescribeAclsResponse{Version: 1, ResourceAcls: []*sarama.ResourceAcls{{
		Resource: sarama.Resource{ResourceType: sarama.AclResourceTopic, ResourceName: "example", ResourcePatternType: sarama.AclPatternLiteral},
		Acls:     []*sarama.Acl{{Principal: "User:Alice", Host: "*", Operation: sarama.AclOperationWrite, PermissionType: sarama.AclPermissionAllow}},
	}}}
	updated := &sarama.DescribeAclsResponse{Version: 1, ResourceAcls: []*sarama.ResourceAcls{{
		Resource: sarama.Resource{ResourceType: sarama.AclResourceTopic, ResourceName: "example", ResourcePatternType: sarama.AclPatternPrefixed},
		Acls:     []*sarama.Acl{{Principal: "User:Alice", Host: "*", Operation: sarama.AclOperationWrite, PermissionType: sarama.AclPermissionDeny}},
	}}}
	unauthorized := &sarama.DescribeAclsResponse{Version: 1, Err: sarama.ErrClusterAuthorizationFailed}
	destroy := func(*terraform.State) error { return testAccCheckAclDestroy("example") }
	mismatch := errors.New("ACL snapshot still mismatched")

	for _, tc := range []struct {
		name         string
		check        func(*terraform.State) error
		snapshots    []*sarama.DescribeAclsResponse
		wantErr      string
		wantRequests int
	}{
		{name: "create", check: testResourceACL_initialCheck, snapshots: []*sarama.DescribeAclsResponse{missing, present}},
		{name: "update", check: testResourceACL_updateCheck, snapshots: []*sarama.DescribeAclsResponse{present, updated}},
		{name: "delete", check: destroy, snapshots: []*sarama.DescribeAclsResponse{present, missing}},
		{name: "create authorization", check: testResourceACL_initialCheck, snapshots: []*sarama.DescribeAclsResponse{unauthorized, present}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "update authorization", check: testResourceACL_updateCheck, snapshots: []*sarama.DescribeAclsResponse{unauthorized, updated}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "delete authorization", check: destroy, snapshots: []*sarama.DescribeAclsResponse{unauthorized, missing}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{
			name: "retryable mismatch",
			check: func(*terraform.State) error {
				err := testACLRetry(testProvider.Meta().(*LazyClient), func([]*sarama.ResourceAcls) error { return mismatch })()
				if err == nil || !err.Retryable || err.Err != mismatch {
					return errors.New("assertion failure was not returned as a retryable error")
				}
				return err.Err
			},
			snapshots: []*sarama.DescribeAclsResponse{missing}, wantErr: mismatch.Error(), wantRequests: 5,
		},
		{
			name: "timeout",
			check: func(*terraform.State) error {
				err := retry.RetryContext(context.Background(), 100*time.Millisecond,
					testACLRetry(testProvider.Meta().(*LazyClient), func([]*sarama.ResourceAcls) error { return mismatch }))
				if err != mismatch {
					return errors.New("timeout did not return the original assertion error")
				}
				return err
			},
			snapshots: []*sarama.DescribeAclsResponse{missing}, wantErr: mismatch.Error(), wantRequests: 5,
		},
		{
			name: "non-retryable authorization",
			check: func(*terraform.State) error {
				checked := false
				err := testACLRetry(testProvider.Meta().(*LazyClient), func([]*sarama.ResourceAcls) error { checked = true; return nil })()
				if err == nil || err.Retryable || checked {
					return errors.New("authorization failure was retried or reached the assertion")
				}
				return err.Err
			},
			snapshots: []*sarama.DescribeAclsResponse{unauthorized, present}, wantErr: sarama.ErrClusterAuthorizationFailed.Error(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var responses []any
			for _, snapshot := range tc.snapshots {
				responses = append(responses, snapshot)
				if snapshot.Err == sarama.ErrNoError {
					responses = append(responses, missing, missing, missing, missing)
				}
			}
			mock := sarama.NewMockBroker(t, 0)
			defer mock.Close()
			mock.SetHandlerByMap(map[string]sarama.MockResponse{
				"MetadataRequest":     sarama.NewMockMetadataResponse(t).SetBroker(mock.Addr(), mock.BrokerID()).SetController(mock.BrokerID()),
				"DescribeAclsRequest": sarama.NewMockSequence(responses...),
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
			lazy := &LazyClient{inner: &Client{client: client, supportedAPIs: map[int]int{29: 1}}}
			lazy.once.Do(func() {})
			originalMeta := testProvider.Meta()
			testProvider.SetMeta(lazy)
			defer testProvider.SetMeta(originalMeta)

			state := &terraform.State{Modules: []*terraform.ModuleState{{Resources: map[string]*terraform.ResourceState{
				"kafka_acl.test": {Primary: &terraform.InstanceState{
					ID: "example",
					Attributes: map[string]string{
						"resource_name": "example",
					},
				}},
			}}}}
			err = tc.check(state)
			if tc.wantErr == "" && err != nil {
				t.Errorf("check failed before metadata converged: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("expected error containing %q, got %v", tc.wantErr, err)
			}
			requests := 0
			for _, exchange := range mock.History() {
				if _, ok := exchange.Request.(*sarama.DescribeAclsRequest); ok {
					requests++
				}
			}
			wantRequests := len(responses)
			if tc.wantErr != "" {
				wantRequests = 1
			}
			if tc.wantRequests != 0 {
				wantRequests = tc.wantRequests
			}
			if requests != wantRequests {
				t.Errorf("got %d describe requests, want %d", requests, wantRequests)
			}
		})
	}
}

func TestACLRetryInitializationError(t *testing.T) {
	wantErr := errors.New("client initialization failed")
	client := &LazyClient{initErr: wantErr}
	client.once.Do(func() {})
	err := testACLRetry(client, func([]*sarama.ResourceAcls) error {
		t.Fatal("assertion called after initialization failed")
		return nil
	})()
	if err == nil || err.Retryable || err.Err != wantErr {
		t.Fatalf("expected non-retryable initialization error, got %+v", err)
	}
}
