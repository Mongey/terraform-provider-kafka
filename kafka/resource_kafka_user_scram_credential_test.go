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
				Config: cfgs(t, bs, fmt.Sprintf(testResourceUserScramCredential_SHA256, username)),
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

func TestAcc_UserScramCredentialWriteOnly(t *testing.T) {
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
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WriteOnly, username)),
				Check:  testResourceUserScramCredentialCheck_writeOnly,
			},
		},
	})
}

func TestAcc_UserScramCredentialWriteOnlyUpdate(t *testing.T) {
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
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WriteOnly, username)),
				Check:  testResourceUserScramCredentialCheck_writeOnly,
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WriteOnlyUpdate, username)),
				Check:  testResourceUserScramCredentialCheck_writeOnlyUpdated,
			},
		},
	})
}

func TestAcc_UserScramCredentialWriteOnlyWithIterations(t *testing.T) {
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
				Config: cfg(t, bs, fmt.Sprintf(testResourceUserScramCredential_WriteOnlyWithIterations, username, 8192)),
				Check:  testResourceUserScramCredentialCheck_writeOnlyWithIterations,
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

func testResourceUserScramCredentialCheck_writeOnly(s *terraform.State) error {
	return testResourceUserScramCredentialCheck_writeOnlyInternal(s, "v1", false)
}

func testResourceUserScramCredentialCheck_writeOnlyUpdated(s *terraform.State) error {
	return testResourceUserScramCredentialCheck_writeOnlyInternal(s, "v2", false)
}

func testResourceUserScramCredentialCheck_writeOnlyWithIterations(s *terraform.State) error {
	return testResourceUserScramCredentialCheck_writeOnlyInternal(s, "v1", true)
}

func testResourceUserScramCredentialCheck_writeOnlyInternal(s *terraform.State, expectedVersion string, withIterations bool) error {
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
	passwordWoVersion := instanceState.Attributes["password_wo_version"]
	expectedIterations := defaultIterations
	if withIterations {
		i, err := strconv.Atoi(instanceState.Attributes["scram_iterations"])
		if err != nil {
			return err
		}
		expectedIterations = int32(i)
	}

	// Check that password_wo_version is set correctly
	if passwordWoVersion != expectedVersion {
		return fmt.Errorf("password_wo_version should be %s but got '%s'", expectedVersion, passwordWoVersion)
	}

	// Verify the credential exists in Kafka (though we can't check the password)
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

	meta := testProvider.Meta()
	if meta == nil {
		return fmt.Errorf("provider Meta() returned nil")
	}

	client := meta.(*LazyClient)
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

const testResourceUserScramCredential_WriteOnly = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-256"
  password_wo            = "write-only-test"
  password_wo_version    = "v1"
}
`

const testResourceUserScramCredential_WriteOnlyUpdate = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-256"
  password_wo            = "write-only-test-updated"
  password_wo_version    = "v2"
}
`

const testResourceUserScramCredential_WriteOnlyWithIterations = `
resource "kafka_user_scram_credential" "test" {
  username               = "%s"
  scram_mechanism        = "SCRAM-SHA-256"
  scram_iterations       = "%d"
  password_wo            = "write-only-test"
  password_wo_version    = "v1"
}
`

// Unit tests for password validation
func Test_getPasswordFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		password    interface{}
		passwordWo  interface{}
		expected    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid password field",
			password:    "valid-password",
			passwordWo:  "",
			expected:    "valid-password",
			expectError: false,
		},
		{
			name:        "valid password_wo field",
			password:    "",
			passwordWo:  "valid-password-wo",
			expected:    "valid-password-wo",
			expectError: false,
		},
		{
			name:        "empty password fields should error",
			password:    "",
			passwordWo:  "",
			expected:    "",
			expectError: true,
			errorMsg:    "either 'password' or 'password_wo' must be provided with a non-empty value",
		},
		{
			name:        "nil password fields should error",
			password:    nil,
			passwordWo:  nil,
			expected:    "",
			expectError: true,
			errorMsg:    "either 'password' or 'password_wo' must be provided with a non-empty value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a resource data instance properly
			resourceSchema := kafkaUserScramCredentialResource()
			d := resourceSchema.TestResourceData()

			// Set the password fields based on test case
			if tt.password != nil {
				if err := d.Set("password", tt.password); err != nil {
					t.Fatalf("failed to set password: %v", err)
				}
			}
			if tt.passwordWo != nil {
				if err := d.Set("password_wo", tt.passwordWo); err != nil {
					t.Fatalf("failed to set password_wo: %v", err)
				}
			}

			result, err := getPasswordFromConfig(d)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result != tt.expected {
					t.Errorf("expected '%s', got '%s'", tt.expected, result)
				}
			}
		})
	}
}

// Test that parseUserScramCredential fails with empty password
func Test_parseUserScramCredential_EmptyPassword(t *testing.T) {
	resourceSchema := kafkaUserScramCredentialResource()
	d := resourceSchema.TestResourceData()

	if err := d.Set("username", "testuser"); err != nil {
		t.Fatalf("failed to set username: %v", err)
	}
	if err := d.Set("scram_mechanism", "SCRAM-SHA-256"); err != nil {
		t.Fatalf("failed to set scram_mechanism: %v", err)
	}
	if err := d.Set("scram_iterations", 4096); err != nil {
		t.Fatalf("failed to set scram_iterations: %v", err)
	}
	// Don't set password fields - they should be empty

	_, err := parseUserScramCredential(d)

	if err == nil {
		t.Error("expected error for empty password, but got none")
		return
	}

	expectedErrorMsg := "either 'password' or 'password_wo' must be provided with a non-empty value"
	if !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("expected error message to contain '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}

// Test that parseUserScramCredential succeeds with valid password
func Test_parseUserScramCredential_ValidPassword(t *testing.T) {
	resourceSchema := kafkaUserScramCredentialResource()
	d := resourceSchema.TestResourceData()

	if err := d.Set("username", "testuser"); err != nil {
		t.Fatalf("failed to set username: %v", err)
	}
	if err := d.Set("scram_mechanism", "SCRAM-SHA-256"); err != nil {
		t.Fatalf("failed to set scram_mechanism: %v", err)
	}
	if err := d.Set("scram_iterations", 4096); err != nil {
		t.Fatalf("failed to set scram_iterations: %v", err)
	}
	if err := d.Set("password", "valid-password"); err != nil {
		t.Fatalf("failed to set password: %v", err)
	}

	credential, err := parseUserScramCredential(d)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if credential.Name != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", credential.Name)
	}

	if string(credential.Password) != "valid-password" {
		t.Errorf("expected password 'valid-password', got '%s'", string(credential.Password))
	}

	if credential.Iterations != 4096 {
		t.Errorf("expected iterations 4096, got %d", credential.Iterations)
	}
}

// Test that parseUserScramCredential succeeds with valid password_wo
func Test_parseUserScramCredential_ValidPasswordWo(t *testing.T) {
	resourceSchema := kafkaUserScramCredentialResource()
	d := resourceSchema.TestResourceData()

	if err := d.Set("username", "testuser"); err != nil {
		t.Fatalf("failed to set username: %v", err)
	}
	if err := d.Set("scram_mechanism", "SCRAM-SHA-512"); err != nil {
		t.Fatalf("failed to set scram_mechanism: %v", err)
	}
	if err := d.Set("scram_iterations", 8192); err != nil {
		t.Fatalf("failed to set scram_iterations: %v", err)
	}
	if err := d.Set("password_wo", "valid-password-wo"); err != nil {
		t.Fatalf("failed to set password_wo: %v", err)
	}

	credential, err := parseUserScramCredential(d)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if credential.Name != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", credential.Name)
	}

	if string(credential.Password) != "valid-password-wo" {
		t.Errorf("expected password 'valid-password-wo', got '%s'", string(credential.Password))
	}

	if credential.Iterations != 8192 {
		t.Errorf("expected iterations 8192, got %d", credential.Iterations)
	}
}
