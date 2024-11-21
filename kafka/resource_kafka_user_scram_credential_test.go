package kafka

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_UserScramCredentialBasic(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	username := fmt.Sprintf("test-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfgs(bs, fmt.Sprintf(testResourceUserScramCredential_SHA256, username)),
				Check:  testResourceUserScramCredentialCheck_withoutIterations,
			},
		},
	})
}

func TestAcc_UserScramCredentialWithIterations(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	username := fmt.Sprintf("test-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WithIterations, username, 8192)),
				Check:  testResourceUserScramCredentialCheck_withIterations,
			},
		},
	})
}

func TestAcc_UserScramCredentialSHA512(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	username := fmt.Sprintf("test-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_SHA512, username)),
				Check:  testResourceUserScramCredentialCheck_withoutIterations,
			},
		},
	})
}

func TestAcc_UserScramCredentialConfigUpdate(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}

	username := fmt.Sprintf("test-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckUserScramCredentialDestroy,
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WithIterations, username, 4096)),
				Check:  testResourceUserScramCredentialCheck_withIterations,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WithIterations, username, 8192)),
				Check:  testResourceUserScramCredentialCheck_withIterations,
			},
		},
	})
}

func testResourceUserScramCredentialCheck_withoutIterations(s *terraform.State) error {
	return testResourceUserScramCredentialCheck(s, false)
}

func testResourceUserScramCredentialCheck_withIterations(s *terraform.State) error {
	return testResourceUserScramCredentialCheck(s, true)
}

func testResourceUserScramCredentialCheck(s *terraform.State, withIterations bool) error {
	resourceState := s.Modules[0].Resources["kafka_user_scram_credential.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	username := instanceState.Attributes["username"]
	scramMechanism := instanceState.Attributes["scram_mechanism"]
	expectedIterations := defaultIterations
	if withIterations {
		i, err := strconv.Atoi(instanceState.Attributes["scram_iterations"])
		if err != nil {
			return err
		}
		expectedIterations = int32(i)
	}

	client := testProvider.Meta().(*LazyClient)
	userScramCredential, err := client.DescribeUserScramCredential(username, scramMechanism)
	if err != nil {
		return err
	}

	id := instanceState.ID
	expectedId := strings.Join([]string{username, scramMechanism}, "|")

	if id != expectedId {
		return fmt.Errorf("id '%s' does not match expected id '%s'", id, expectedId)
	}

	if userScramCredential.Iterations != expectedIterations {
		return fmt.Errorf("scram iterations should be %d but got '%d'", expectedIterations, userScramCredential.Iterations)
	}

	return nil
}

func testAccCheckUserScramCredentialDestroy(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_user_scram_credential.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	username := instanceState.Attributes["username"]
	mechanism := instanceState.Attributes["scram_mechanism"]

	client := testProvider.Meta().(*LazyClient)
	_, err := client.DescribeUserScramCredential(username, mechanism)

	if _, ok := err.(UserScramCredentialMissingError); !ok {
		if err == nil {
			return fmt.Errorf("user scram credential was not destroyed")
		} else {
			return fmt.Errorf("user scram credential was not destroyed %v", err.Error())
		}
	}

	return nil
}

const testResourceUserScramCredential_SHA256 = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-256"
  password               = "test"
}
`

const testResourceUserScramCredential_SHA512 = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-512"
  password               = "test"
}
`
const testResourceUserScramCredential_WithIterations = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-256"
  scram_iterations       = "%d"
  password               = "test"
}
`
