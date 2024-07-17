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

func TestConfigFailOn(t *testing.T) {
	// Test with a value
	cfg := &Config{FailOn: []string{"partition_lower"}}
	if len(cfg.FailOn) != 1 {
		t.Fatalf("expected 1 fail_on condition, got %d", len(cfg.FailOn))
	}
	if !Contains(cfg.FailOn, "partition_lower") {
		t.Fatalf("expected fail_on condition 'partition_lower', got %s", cfg.FailOn[0])
	}
}

func TestConfigFailOnEmpty(t *testing.T) {
	// Test with an empty value
	cfgEmpty := &Config{FailOn: []string{}}
	if len(cfgEmpty.FailOn) != 0 {
		t.Fatalf("expected 0 fail_on conditions, got %d", len(cfgEmpty.FailOn))
	}
	if cfgEmpty == nil {
		t.Fatal("expected non-nil config")
	}
}
