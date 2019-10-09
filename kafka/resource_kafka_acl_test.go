package kafka

import (
	"fmt"
	"log"
	"testing"

	"github.com/Shopify/sarama"
	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAcc_ACLCreateAndUpdate(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	aclResourceName := fmt.Sprintf("syslog-%s", u)

	r.Test(t, r.TestCase{
		Providers:  accProvider(),
		IsUnitTest: false,
		PreCheck:   func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(testResourceACL_initialConfig, aclResourceName),
				Check:  testResourceACL_initialCheck,
			},
			{
				Config: fmt.Sprintf(testResourceACL_updateConfig, aclResourceName),
				Check:  testResourceACL_updateCheck,
			},
		},
	})
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

	client := testProvider.Meta().(*Client)
	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	if len(acls) < 1 {
		return fmt.Errorf("There should be one acls %v %s", acls, err)
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

	if acl.Resource.ResoucePatternType != sarama.AclPatternLiteral {
		return fmt.Errorf("Should be Literal, not %v", acl.Resource.ResoucePatternType)
	}
	return nil
}

func testResourceACL_updateCheck(s *terraform.State) error {
	client := testProvider.Meta().(*Client)

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

	//if acl.ResourceName != "syslog" {
	//return fmt.Errorf("The expected ACL should be for syslog")
	//}

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
		return fmt.Errorf("Should be Deny, not %v", acl.Acls[0].PermissionType)
	}

	if acl.Resource.ResoucePatternType != sarama.AclPatternPrefixed {
		return fmt.Errorf("Should be Prefixed, not %v", acl.Resource.ResoucePatternType)
	}
	return nil
}

const testResourceACL_initialConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
	ca_cert           = file("../secrets/snakeoil-ca-1.crt")
	client_cert       = file("../secrets/kafkacat-ca1-signed.pem")
	client_key        = file("../secrets/kafkacat-raw-private-key.pem")
	skip_tls_verify   = true
}

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
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
	ca_cert           = file("../secrets/snakeoil-ca-1.crt")
	client_cert       = file("../secrets/kafkacat-ca1-signed.pem")
	client_key        = file("../secrets/kafkacat-raw-private-key.pem")
	skip_tls_verify   = true
}

resource "kafka_acl" "test" {
	resource_name       = "%s"
	resource_type       = "Topic"
	resource_pattern_type_filter = "Prefixed"
	acl_principal       = "User:Alice"
	acl_host            = "*"
	acl_operation       = "Write"
	acl_permission_type = "Deny"
}
`
