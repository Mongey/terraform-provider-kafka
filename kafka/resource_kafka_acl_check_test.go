package kafka

import (
	"strings"
	"testing"

	"github.com/IBM/sarama"
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

	for _, tc := range []struct {
		name      string
		check     func(*terraform.State) error
		snapshots []*sarama.DescribeAclsResponse
		wantErr   string
	}{
		{name: "create", check: testResourceACL_initialCheck, snapshots: []*sarama.DescribeAclsResponse{missing, present}},
		{name: "update", check: testResourceACL_updateCheck, snapshots: []*sarama.DescribeAclsResponse{present, updated}},
		{name: "delete", check: destroy, snapshots: []*sarama.DescribeAclsResponse{present, missing}},
		{name: "create authorization", check: testResourceACL_initialCheck, snapshots: []*sarama.DescribeAclsResponse{unauthorized, present}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "update authorization", check: testResourceACL_updateCheck, snapshots: []*sarama.DescribeAclsResponse{unauthorized, updated}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
		{name: "delete authorization", check: destroy, snapshots: []*sarama.DescribeAclsResponse{unauthorized, missing}, wantErr: sarama.ErrClusterAuthorizationFailed.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var responses []any
			for _, snapshot := range tc.snapshots {
				responses = append(responses, snapshot)
				if snapshot.Err == sarama.ErrNoError {
					// ListACLs describes topics, groups, clusters, transactions, and tokens.
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
			if requests != wantRequests {
				t.Errorf("got %d describe requests, want %d", requests, wantRequests)
			}
		})
	}
}
