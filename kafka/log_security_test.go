package kafka

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func captureSecurityLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

func TestProviderCredentialLogs(t *testing.T) {
	buf := captureSecurityLogs(t)
	secret := "credential-sentinel"
	p := Provider()
	values := map[string]interface{}{"bootstrap_servers": []interface{}{"localhost:9092"}}
	for _, name := range []string{
		"client_key", "client_key_file", "client_key_passphrase", "sasl_password",
		"sasl_aws_external_id", "sasl_aws_access_key", "sasl_aws_secret_key",
		"sasl_aws_token", "sasl_token_url", "sasl_aws_container_credentials_full_uri",
	} {
		if !p.Schema[name].Sensitive {
			t.Errorf("%s must be sensitive", name)
		}
		values[name] = secret
	}
	values["client_cert"] = secret
	values["ca_cert"] = secret
	values["sasl_username"] = secret
	values["sasl_aws_container_authorization_token_file"] = secret
	meta, err := providerConfigure(schema.TestResourceDataRaw(t, p.Schema, values))
	if err != nil {
		t.Fatal(err)
	}
	client := meta.(*LazyClient)
	before := *client.Config
	if err := client.init(); err == nil {
		t.Fatal("expected invalid certificate to fail initialization")
	} else if strings.Contains(err.Error(), secret) {
		t.Fatal("initialization error contains credential")
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatal("provider or lazy client log contains credential")
	}
	if !reflect.DeepEqual(before, *client.Config) {
		t.Fatal("logging mutated client configuration")
	}
}

func TestMalformedPEMDoesNotLeak(t *testing.T) {
	buf := captureSecurityLogs(t)
	secret := "-----BEGIN PRIVATE KEY-----\nprivate-key-sentinel\ninvalid"
	_, _, err := parsePemOrLoadFromFile(secret)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file-not-found cause, got %v", err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Path != secret {
		t.Fatal("PEM error must preserve the original PathError")
	}
	if strings.Contains(fmt.Sprint(err)+buf.String(), "private-key-sentinel") {
		t.Fatal("malformed PEM leaked through logs or error")
	}
}

func TestOAuthErrorsDoNotLeak(t *testing.T) {
	secret := "oauth-secret-sentinel"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := fmt.Fprint(w, secret); err != nil {
			t.Errorf("write OAuth test response: %v", err)
		}
	}))
	defer server.Close()
	for _, endpoint := range []string{server.URL + "?secret=" + secret, "://" + secret} {
		provider := newOauthbearerTokenProvider(&clientcredentials.Config{
			TokenURL: endpoint, ClientID: "client", ClientSecret: secret,
		})
		token, err := provider.Token()
		if err == nil || strings.Contains(fmt.Errorf("request failed: %w", err).Error(), secret) || token.Token != "" {
			t.Fatal("OAuth failure must return an empty token and a sanitized error")
		}
		var retrieveErr *oauth2.RetrieveError
		var urlErr *url.Error
		if !errors.As(err, &retrieveErr) && !errors.As(err, &urlErr) {
			t.Fatal("OAuth error must preserve its typed cause")
		}
	}
}

func TestAWSTokenFileErrorDoesNotLeak(t *testing.T) {
	secret := "aws-token-path-sentinel"
	config := Config{
		SASLAWSContainerAuthorizationTokenFile: t.TempDir() + "/" + secret,
		SASLAWSContainerCredentialsFullUri:     "http://localhost/" + secret,
	}
	_, err := config.Token()
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatal("AWS token file failure must return a sanitized error")
	}
	var pathErr *os.PathError
	if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) {
		t.Fatal("AWS token file error must preserve its cause")
	}
}

type securityConfigClient struct {
	sarama.Client
	broker *sarama.Broker
}

func (c securityConfigClient) Controller() (*sarama.Broker, error)     { return c.broker, nil }
func (c securityConfigClient) RefreshMetadata(...string) error         { return nil }
func (c securityConfigClient) Topics() ([]string, error)               { return []string{"example"}, nil }
func (c securityConfigClient) Partitions(string) ([]int32, error)      { return []int32{0}, nil }
func (c securityConfigClient) Replicas(string, int32) ([]int32, error) { return []int32{0}, nil }

func TestTopicConfigLogsOmitValues(t *testing.T) {
	buf := captureSecurityLogs(t)
	secret := "topic-config-secret-sentinel"
	mock := sarama.NewMockBroker(t, 0)
	defer mock.Close()
	mock.SetHandlerByMap(map[string]sarama.MockResponse{
		"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{
			Version: 2, Resources: []*sarama.ResourceResponse{{Configs: []*sarama.ConfigEntry{{
				Name: "custom.config", Value: secret, Source: sarama.SourceTopic,
				Synonyms: []*sarama.ConfigSynonym{{ConfigName: "custom.config", ConfigValue: secret}},
			}}}},
		}),
	})
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_0_0_0
	cfg.ApiVersionsRequest = false
	broker := sarama.NewBroker(mock.Addr())
	if err := broker.Open(cfg); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := broker.Close(); err != nil {
			t.Errorf("close test broker: %v", err)
		}
	}()
	client := &Client{client: securityConfigClient{broker: broker}, kafkaConfig: cfg, config: &Config{}}
	got, err := client.topicConfig("example")
	if err != nil || got["custom.config"] == nil || *got["custom.config"] != secret {
		t.Fatalf("config retrieval changed: %v", err)
	}
	lazy := &LazyClient{inner: client}
	lazy.once.Do(func() {})
	expected := Topic{Name: "example", Config: map[string]*string{"custom.config": &secret}}
	result, state, err := topicRefreshFunc(lazy, "example", expected)()
	if err != nil || state != "Updating" || result == nil {
		t.Fatalf("expected sanitized pending state, got %q, error %v", state, err)
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatal("topic configuration leaked through logs")
	}
}

func TestScramCredentialLogsOmitPassword(t *testing.T) {
	credential := UserScramCredential{Name: "user", Password: []byte("scram-secret-sentinel")}
	for _, format := range []string{"%v", "%+v", "%s"} {
		if strings.Contains(fmt.Sprintf(format, credential), "scram-secret-sentinel") {
			t.Fatal("SCRAM credential formatting exposed password")
		}
	}
}
