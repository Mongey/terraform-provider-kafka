package kafka

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/IBM/sarama"
	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ACLCreateAndUpdate(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	aclResourceName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      func(s *terraform.State) error { return testAccCheckAclDestroy(aclResourceName) },
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceACL_initialConfig, aclResourceName)),
				Check:  testResourceACL_initialCheck,
			},
			{
				Config:   cfg(t, bs, fmt.Sprintf(testResourceACL_initialConfig, aclResourceName)),
				PlanOnly: true,
				PreConfig: func() {
					client := testProvider.Meta().(*LazyClient)
					acl := StringlyTypedACL{
						ACL{
							Principal:      "User:Alice",
							Host:           "*",
							Operation:      "Write",
							PermissionType: "Allow",
						},
						Resource{
							Type:              "Topic",
							Name:              aclResourceName,
							PatternTypeFilter: "Literal",
						},
					}
					err := client.DeleteACL(acl)
					if err != nil {
						t.Fatal(err)
					}
				},
				ExpectNonEmptyPlan: true,
				Check: func(s *terraform.State) error {
					if len(s.Modules[0].Resources) != 0 {
						return fmt.Errorf("Expected no resources to exist got %d", len(s.Modules[0].Resources))
					}
					return nil
				},
			},
		},
	})
}

func TestAcc_ACLDeletedOutsideOfTerraform(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	aclResourceName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      func(s *terraform.State) error { return testAccCheckAclDestroy(aclResourceName) },
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceACL_initialConfig, aclResourceName)),
				Check:  testResourceACL_initialCheck,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceACL_updateConfig, aclResourceName)),
				Check:  testResourceACL_updateCheck,
			},
			{
				ResourceName:      "kafka_acl.test",
				ImportState:       true,
				ImportStateVerify: true,
				Config:            cfg(t, bs, fmt.Sprintf(testResourceACL_updateConfig, aclResourceName)),
			},
		},
	})
}

func testAccCheckAclDestroy(name string) error {
	client := testProvider.Meta().(*LazyClient)
	err := client.InvalidateACLCache()
	if err != nil {
		return err
	}
	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	log.Printf("[INFO] Searching for the ACL with resource_name %s", name)

	aclCount := 0
	for _, searchACL := range acls {
		if searchACL.ResourceName == name {
			log.Printf("[INFO] Found acl with resource_name %s : %v", name, searchACL)
			aclCount++
		}
	}
	if aclCount != 0 {
		return fmt.Errorf("Expected 0 acls for ACL %s, got %d", name, aclCount)
	}
	return nil
}

func testResourceACL_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_acl.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	client := testProvider.Meta().(*LazyClient)
	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	if len(acls) < 1 {
		return fmt.Errorf("There should be one acl, got %d, %v %s", len(acls), acls, err)
	}

	name := instanceState.Attributes["resource_name"]
	log.Printf("[INFO] Searching for the ACL with resource_name %s", name)
	acl := acls[0]
	aclCount := 0
	for _, searchACL := range acls {
		if searchACL.ResourceName == name {
			log.Printf("[INFO] Found acl with resource_name %s : %v", name, searchACL)
			acl = searchACL
			aclCount++
		}
	}

	if acl.Acls[0].PermissionType != sarama.AclPermissionAllow {
		return fmt.Errorf("Should be Allow, not %v", acl.Acls[0].PermissionType)
	}

	if acl.Resource.ResourcePatternType != sarama.AclPatternLiteral {
		return fmt.Errorf("Should be Literal, not %v", acl.Resource.ResourcePatternType)
	}
	log.Printf("[INFO] success")
	return nil
}

func testResourceACL_updateCheck(s *terraform.State) error {
	client := testProvider.Meta().(*LazyClient)
	err := client.InvalidateACLCache()
	if err != nil {
		return err
	}
	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	if len(acls) < 1 {
		return fmt.Errorf("There should be some acls %v %s", acls, err)
	}

	resourceState := s.Modules[0].Resources["kafka_acl.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}
	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	name := instanceState.Attributes["resource_name"]
	log.Printf("[INFO] Searching for the ACL with resource_name %s", name)

	aclCount := 0
	acl := acls[0]
	for _, searchACL := range acls {
		if searchACL.ResourceName == name {
			log.Printf("[INFO] Found acl with resource_name %s : %v", name, searchACL)
			acl = searchACL
			aclCount++
		}
	}

	if len(acl.Acls) != 1 {
		return fmt.Errorf("There are %d ACLs when there should be 1: %v", len(acl.Acls), acl.Acls)
	}
	if aclCount != 1 {
		return fmt.Errorf("There should only be one acl with this resource, but there are %d", aclCount)
	}
	if acl.ResourceType != sarama.AclResourceTopic {
		return fmt.Errorf("Should be for a topic")
	}

	if acl.Acls[0].Principal != "User:Alice" {
		return fmt.Errorf("Should be for Alice")
	}

	if acl.Acls[0].Host != "*" {
		return fmt.Errorf("Should be for *")
	}
	if acl.Acls[0].PermissionType != sarama.AclPermissionDeny {
		return fmt.Errorf("should be Deny, not %v", acl.Acls[0].PermissionType.String())
	}

	if acl.Resource.ResourcePatternType != sarama.AclPatternPrefixed {
		return fmt.Errorf("Should be Prefixed, not %v", acl.Resource.ResourcePatternType)
	}
	return nil
}

const testResourceACL_initialConfig = `
resource "kafka_acl" "test" {
	resource_name       = "%s"
	resource_type       = "Topic"
	resource_pattern_type_filter = "Literal"
	acl_principal       = "User:Alice"
	acl_host            = "*"
	acl_operation       = "Write"
	acl_permission_type = "Allow"
}
`

const testResourceACL_updateConfig = `
resource "kafka_acl" "test" {
	resource_name                = "%s"
	resource_type                = "Topic"
	resource_pattern_type_filter = "Prefixed"
	acl_principal                = "User:Alice"
	acl_host                     = "*"
	acl_operation                = "Write"
	acl_permission_type          = "Deny"
}
`

// lintignore:AT004
func cfg(t *testing.T, bs string, extraCfg string) string {
	_, err := os.ReadFile("../secrets/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.ReadFile("../secrets/terraform-cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.ReadFile("../secrets/terraform.pem")
	if err != nil {
		t.Fatal(err)
	}

	return fmt.Sprintf(`
provider "kafka" {
	bootstrap_servers = ["%s"]
}

%s
`, bs, extraCfg)
}
