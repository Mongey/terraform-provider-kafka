package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_BasicUserScramCredential(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("test1-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfgs(t, bs, fmt.Sprintf(testResourceUserScramCredential, username, "SCRAM-SHA-256")),
				Check:  testResourceUserScramCredential_initialCheck,
			},
		},
	})
}

func TestAcc_UserScramCredentialConfigUpdate(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("test2-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential, username, "SCRAM-SHA-256")),
				Check:  testResourceUserScramCredential_initialCheck,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential, username, "SCRAM-SHA-512")),
				Check:  testResourceUserScramCredential_updateCheck,
			},
		},
	})
}

func testResourceUserScramCredential_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_user_scram_credential.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}
	username := instanceState.Attributes["username"]
	scramMechanism := instanceState.Attributes["scram_mechanism"]
	scramIterations := instanceState.Attributes["scram_iterations"]

	client := testProvider.Meta().(*LazyClient)
	userScramCredential, err := client.DescribeUserScramCredential(username)
	if err != nil {
		return err
	}

	id := instanceState.ID

	if id != username {
		return fmt.Errorf("id doesn't match for %s, got %s", id, username)
	}

	if string(userScramCredential.Iterations) != scramIterations {
		return fmt.Errorf("scram iterations should be 4096 but got '%v'", userScramCredential.Iterations)
	}

	if userScramCredential.Mechanism.String() != scramMechanism {
		return fmt.Errorf("scram iterations should be %s but got '%v'", scramMechanism, userScramCredential.Mechanism.String())
	}

	if userScramCredential.Password != "test" {
		return fmt.Errorf("password should be 'test' but got '%v'", userScramCredential.Password)
	}

	return nil
}

func testResourceUserScramCredential_updateCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_user_scram_credential.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	username := instanceState.Attributes["username"]
	scramMechanism := instanceState.Attributes["scram_mechanism"]

	client := testProvider.Meta().(*LazyClient)
	userScramCredential, err := client.DescribeUserScramCredential(username)
	if err != nil {
		return err
	}

	id := instanceState.ID

	if id != username {
		return fmt.Errorf("id doesn't match for %s, got %s", id, username)
	}

	if userScramCredential.Mechanism.String() != scramMechanism {
		return fmt.Errorf("scram iterations should be %s but got '%v'", scramMechanism, userScramCredential.Mechanism.String())
	}

	return nil
}

func testAccCheckUserScramCredentialDestroy(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_user_scram_credential.test1"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	username := instanceState.Attributes["username"]

	client := testProvider.Meta().(*LazyClient)
	_, err := client.DescribeUserScramCredential(username)

	if _, ok := err.(UserScramCredentialMissingError); !ok {
		return fmt.Errorf("user scram credential was not found %v", err.Error())
	}

	return nil
}


const testResourceUserScramCredential = `
resource "kafka_user_scram_credential" "test1" {
  provider = "kafka"
  username               = "%s"
  scram_mechanism        = "%s"
  scram_iterations		 = 4096
  password 				 = "test"
}
`
