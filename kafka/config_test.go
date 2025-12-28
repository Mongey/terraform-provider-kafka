package kafka

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func assertEquals(t *testing.T, expected any, actual any) {
	if expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func assertNil(t *testing.T, actual any) {
	assertEquals(t, nil, actual)
}

func assertNotNil(t *testing.T, actual any) {
	if actual == nil {
		t.Error("Actual was nil")
	}
}

func Test_newOauthbearerTokenProvider(t *testing.T) {
	tokenUrl := "https://fake-url.com/token"
	clientId := "clientId"
	clientSecret := "clientSecret"
	oauth2Config := clientcredentials.Config{
		TokenURL:     tokenUrl,
		ClientID:     clientId,
		ClientSecret: clientSecret,
	}
	tokenProvider := newOauthbearerTokenProvider(&oauth2Config)
	//assertEquals(t, tokenUrl, tokenProvider.oauth2Config.TokenURL)
	//assertEquals(t, clientId, tokenProvider.oauth2Config.ClientID)
	//assertEquals(t, clientSecret, tokenProvider.oauth2Config.ClientSecret)
	assertEquals(t, "", tokenProvider.token)
	assertEquals(t, time.Time{}, tokenProvider.tokenExpiration)
}

type MockConfig_NoError struct {
	AccessToken string
	Expiry      time.Time
}

type MockConfig_Error struct {
	err error
}

func (m *MockConfig_NoError) Token(ctx context.Context) (*oauth2.Token, error) {
	// You can customize the mock behavior for testing here.
	return &oauth2.Token{
		AccessToken: m.AccessToken,
		Expiry:      m.Expiry,
	}, nil
}

func (m *MockConfig_Error) Token(ctx context.Context) (*oauth2.Token, error) {
	return nil, m.err
}

func TestOauthbearerTokenProvider_Token_WhenNoPreviousTokenExists(t *testing.T) {
	now := time.Now()
	mockConfig := MockConfig_NoError{
		AccessToken: "tokenNew",
		Expiry:      now,
	}
	tokenProvider := newOauthbearerTokenProvider(&mockConfig)

	token, err := tokenProvider.Token()

	assertNil(t, err)

	assertEquals(t, mockConfig.AccessToken, token.Token)
	assertEquals(t, mockConfig.AccessToken, tokenProvider.token)
	assertEquals(t, mockConfig.Expiry, tokenProvider.tokenExpiration)

}

func TestOauthbearerTokenProvider_Token_WhenPreviousTokenExistsButExpired(t *testing.T) {
	now := time.Now()
	mockConfig := MockConfig_NoError{
		AccessToken: "tokenNew",
		Expiry:      now.Add(time.Duration(24) * time.Hour),
	}
	tokenProvider := newOauthbearerTokenProvider(&mockConfig)
	tokenProvider.token = "tokenOld"
	tokenProvider.tokenExpiration = now.Add(time.Duration(-10) * time.Second)

	token, err := tokenProvider.Token()

	assertNil(t, err)

	assertEquals(t, mockConfig.AccessToken, token.Token)
	assertEquals(t, mockConfig.AccessToken, tokenProvider.token)
	assertEquals(t, mockConfig.Expiry, tokenProvider.tokenExpiration)

}

func TestOauthbearerTokenProvider_Token_WhenPreviousTokenExists(t *testing.T) {
	mockConfig := MockConfig_NoError{
		AccessToken: "tokenNew",
		Expiry:      time.Now(),
	}
	oldToken := "tokenOld"
	expiry := time.Now().Add(time.Duration(100) * time.Second)
	tokenProvider := newOauthbearerTokenProvider(&mockConfig)
	tokenProvider.token = oldToken
	tokenProvider.tokenExpiration = expiry

	token, err := tokenProvider.Token()

	assertNil(t, err)

	assertEquals(t, oldToken, token.Token)
	assertEquals(t, oldToken, tokenProvider.token)
	assertEquals(t, expiry, tokenProvider.tokenExpiration)

}

func TestOauthbearerTokenProvider_Token_WhenError(t *testing.T) {
	mockConfig := MockConfig_Error{
		err: errors.New("TestError"),
	}
	tokenProvider := newOauthbearerTokenProvider(&mockConfig)

	token, err := tokenProvider.Token()

	assertEquals(t, "", token.Token)

	assertEquals(t, mockConfig.err, err)
}

func TestConfig_NewKafkaConfig_WithOauthBearerMechanism(t *testing.T) {
	user := "user"
	pass := "pass"
	url := "url"
	mechanism := "oauthbearer"
	config := Config{
		SASLUsername:  user,
		SASLPassword:  pass,
		SASLTokenUrl:  url,
		SASLMechanism: mechanism,
	}

	sConfig, err := config.newKafkaConfig()

	assertNil(t, err)

	assertEquals(t, user, sConfig.Net.SASL.User)
	assertEquals(t, pass, sConfig.Net.SASL.Password)
	assertNotNil(t, sConfig.Net.SASL.TokenProvider)
	assertEquals(t, sarama.SASLMechanism(sarama.SASLTypeOAuth), sConfig.Net.SASL.Mechanism)
}

func loadFile(t *testing.T, file string) string {
	fb, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("[ERROR] Error reading file %s", err)
	}
	return string(fb)
}

func Test_newTLSConfig(t *testing.T) {
	type args struct {
		clientCert          string
		clientKey           string
		caCert              string
		clientKeyPassphrase string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no special certs",
			args: args{
				clientCert:          "",
				clientKey:           "",
				caCert:              "",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key files",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client-no-password.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key files without passphrase",
			args: args{
				clientCert:          "../secrets/client.pem",
				clientKey:           "../secrets/client-no-password.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "wrong",
			},
			wantErr: false,
		},
		{
			name: "unencrypted key content without passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           loadFile(t, "../secrets/client-no-password.key"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           loadFile(t, "../secrets/client.key"),
				caCert:              loadFile(t, "../secrets/ca.crt"),
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "encrypted key content with passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "test-pass",
			},
			wantErr: false,
		},
		{
			name: "encrypted cert content without passphrase and mixed file/content load",
			args: args{
				clientCert:          loadFile(t, "../secrets/client.pem"),
				clientKey:           "../secrets/client.key",
				caCert:              "../secrets/ca.crt",
				clientKeyPassphrase: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTLSConfig(tt.args.clientCert, tt.args.clientKey, tt.args.caCert, tt.args.clientKeyPassphrase)
			if (err != nil) != tt.wantErr {
				t.Errorf("newTLSConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConfig_NewKafkaConfig_InvalidSASLMechanism(t *testing.T) {
	config := Config{
		SASLUsername:  "user",
		SASLPassword:  "pass",
		SASLMechanism: "invalid-mechanism",
	}

	_, err := config.newKafkaConfig()

	if err == nil {
		t.Fatal("Expected error for invalid SASL mechanism, got nil")
	}

	expectedMsg := `invalid sasl mechanism "invalid-mechanism": can only be "scram-sha256", "scram-sha512", "aws-iam", "oauthbearer" or "plain"`
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestConfig_NewKafkaConfig_AWSIAMMissingRegion(t *testing.T) {
	// Clear AWS_REGION env var if set
	originalRegion := os.Getenv("AWS_REGION")
	os.Unsetenv("AWS_REGION")
	defer func() {
		if originalRegion != "" {
			os.Setenv("AWS_REGION", originalRegion)
		}
	}()

	config := Config{
		SASLMechanism: "aws-iam",
	}

	_, err := config.newKafkaConfig()

	if err == nil {
		t.Fatal("Expected error for missing AWS region, got nil")
	}

	expectedMsg := "aws region must be configured or AWS_REGION environment variable must be set to use aws-iam sasl mechanism"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestConfig_NewKafkaConfig_OAuthBearerMissingTokenURL(t *testing.T) {
	// Clear TOKEN_URL env var if set
	originalTokenURL := os.Getenv("TOKEN_URL")
	os.Unsetenv("TOKEN_URL")
	defer func() {
		if originalTokenURL != "" {
			os.Setenv("TOKEN_URL", originalTokenURL)
		}
	}()

	config := Config{
		SASLUsername:  "user",
		SASLPassword:  "pass",
		SASLMechanism: "oauthbearer",
		SASLTokenUrl:  "", // Empty token URL
	}

	_, err := config.newKafkaConfig()

	if err == nil {
		t.Fatal("Expected error for missing token URL, got nil")
	}

	expectedMsg := "token url must be configured or TOKEN_URL environment variable must be set to use oauthbearer sasl mechanism"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

// TestNewTLSConfig_LegacyEncryptedPEM verifies that legacy PEM-encrypted private keys
// (using the deprecated Proc-Type: 4,ENCRYPTED format) are still supported for backward
// compatibility. This format uses x509.IsEncryptedPEMBlock and x509.DecryptPEMBlock
// which are deprecated since Go 1.16, but we maintain support for users with existing
// encrypted keys generated using tools like:
//
//	openssl genrsa -des3 -out client.key 4096
//
// Note: For new deployments, PKCS#8 encrypted keys are recommended.
// See: https://go.dev/doc/go1.16#crypto/x509
func TestNewTLSConfig_LegacyEncryptedPEM(t *testing.T) {
	t.Run("decrypts legacy encrypted PEM key with correct passphrase", func(t *testing.T) {
		clientCert := loadFile(t, "../secrets/client.pem")
		clientKey := loadFile(t, "../secrets/client.key") // Legacy encrypted key
		caCert := loadFile(t, "../secrets/ca.crt")
		passphrase := "test-pass"

		tlsConfig, err := newTLSConfig(clientCert, clientKey, caCert, passphrase)

		if err != nil {
			t.Fatalf("Expected no error decrypting legacy PEM, got: %v", err)
		}
		if len(tlsConfig.Certificates) == 0 {
			t.Error("Expected at least one certificate to be loaded")
		}
	})

	t.Run("fails with wrong passphrase for legacy encrypted PEM key", func(t *testing.T) {
		clientCert := loadFile(t, "../secrets/client.pem")
		clientKey := loadFile(t, "../secrets/client.key") // Legacy encrypted key
		caCert := loadFile(t, "../secrets/ca.crt")
		wrongPassphrase := "wrong-password"

		_, err := newTLSConfig(clientCert, clientKey, caCert, wrongPassphrase)

		if err == nil {
			t.Error("Expected error when using wrong passphrase for encrypted key")
		}
	})

	t.Run("fails when no passphrase provided for legacy encrypted PEM key", func(t *testing.T) {
		clientCert := loadFile(t, "../secrets/client.pem")
		clientKey := loadFile(t, "../secrets/client.key") // Legacy encrypted key
		caCert := loadFile(t, "../secrets/ca.crt")

		_, err := newTLSConfig(clientCert, clientKey, caCert, "")

		if err == nil {
			t.Error("Expected error when no passphrase provided for encrypted key")
		}
	})

	t.Run("works with unencrypted key without passphrase", func(t *testing.T) {
		clientCert := loadFile(t, "../secrets/client.pem")
		clientKey := loadFile(t, "../secrets/client-no-password.key") // Unencrypted key
		caCert := loadFile(t, "../secrets/ca.crt")

		tlsConfig, err := newTLSConfig(clientCert, clientKey, caCert, "")

		if err != nil {
			t.Fatalf("Expected no error with unencrypted key, got: %v", err)
		}
		if len(tlsConfig.Certificates) == 0 {
			t.Error("Expected at least one certificate to be loaded")
		}
	})
}

func BenchmarkIsAWSMSKServerless(b *testing.B) {
	servers := []string{"kafka-serverless.us-east-1.amazonaws.com"}
	config := &Config{BootstrapServers: &servers}
	for i := 0; i < b.N; i++ {
		config.isAWSMSKServerless()
	}
}

func TestConfig_isAWSMSKServerless(t *testing.T) {
	tests := []struct {
		name             string
		bootstrapServers []string
		want             bool
	}{
		{
			name:             "AWS MSK Serverless endpoint - lowercase",
			bootstrapServers: []string{"kafka-serverless.us-east-1.amazonaws.com:9092"},
			want:             true,
		},
		{
			name:             "AWS MSK Serverless endpoint - mixed case",
			bootstrapServers: []string{"Kafka-Serverless.eu-west-1.amazonaws.com:9092"},
			want:             true,
		},
		{
			name:             "AWS MSK Serverless endpoint - uppercase",
			bootstrapServers: []string{"KAFKA-SERVERLESS.ap-southeast-1.AMAZONAWS.COM:9092"},
			want:             true,
		},
		{
			name:             "Multiple servers with one MSK Serverless",
			bootstrapServers: []string{
				"regular-kafka:9092",
				"kafka-serverless.us-west-2.amazonaws.com:9092",
				"another-kafka:9092",
			},
			want:             true,
		},
		{
			name:             "Regular MSK endpoint (not serverless)",
			bootstrapServers: []string{"b-1.mycluster.xyz123.c4.kafka.us-east-1.amazonaws.com:9092"},
			want:             false,
		},
		{
			name:             "Non-AWS endpoint",
			bootstrapServers: []string{"localhost:9092"},
			want:             false,
		},
		{
			name:             "Multiple non-MSK-serverless servers",
			bootstrapServers: []string{
				"kafka1:9092",
				"kafka2:9092",
				"b-1.cluster.xyz.kafka.us-east-1.amazonaws.com:9092",
			},
			want:             false,
		},
		{
			name:             "Empty bootstrap servers",
			bootstrapServers: []string{},
			want:             false,
		},
		{
			name:             "Partial match but not valid pattern",
			bootstrapServers: []string{"kafka-serverless-test.amazonaws.com:9092"},
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BootstrapServers: &tt.bootstrapServers,
			}
			if got := config.isAWSMSKServerless(); got != tt.want {
				t.Errorf("Config.isAWSMSKServerless() = %v, want %v", got, tt.want)
			}
		})
	}
}
