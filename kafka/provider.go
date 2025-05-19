package kafka

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"bootstrap_servers": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Required:    true,
				Description: "A list of kafka brokers",
			},
			"ca_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CA_CERT", nil),
				Description: "Path to a CA certificate file to validate the server's certificate.",
				Deprecated:  "This parameter is now deprecated and will be removed in a later release, please use `ca_cert` instead.",
			},
			"client_cert_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_CERT", nil),
				Description: "Path to a file containing the client certificate.",
				Deprecated:  "This parameter is now deprecated and will be removed in a later release, please use `client_cert` instead.",
			},
			"client_key_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_KEY", nil),
				Description: "Path to a file containing the private key that the certificate was issued for.",
				Deprecated:  "This parameter is now deprecated and will be removed in a later release, please use `client_key` instead.",
			},
			"ca_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CA_CERT", nil),
				Description: "CA certificate file to validate the server's certificate.",
			},
			"client_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_CERT", nil),
				Description: "The client certificate.",
			},
			"client_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_KEY", nil),
				Description: "The private key that the certificate was issued for.",
			},
			"client_key_passphrase": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_KEY_PASSPHRASE", nil),
				Description: "The passphrase for the private key that the certificate was issued for.",
			},
			"sasl_aws_region": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_IAM_AWS_REGION", nil),
				Description: "AWS region where MSK is deployed.",
			},
			"kafka_version": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_VERSION", "2.7.0"),
				Description: "The version of Kafka protocol to use in `$MAJOR.$MINOR.$PATCH` format. Some features may not be available on older versions. Default is 2.7.0.",
			},
			"sasl_aws_container_authorization_token_file": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE", nil),
				Description: "Path to a file containing the AWS pod identity authorization token",
			},
			"sasl_aws_container_credentials_full_uri": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_CONTAINER_CREDENTIALS_FULL_URI", nil),
				Description: "URI to retrieve AWS credentials from",
			},
			"sasl_aws_role_arn": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_ROLE_ARN", nil),
				Description: "Arn of an AWS IAM role to assume",
			},
			"sasl_aws_external_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "External ID of the AWS IAM role to assume",
			},
			"sasl_aws_profile": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_PROFILE", nil),
				Description: "AWS profile name to use",
			},
			"sasl_aws_access_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_ACCESS_KEY_ID", nil),
				Description: "The AWS access key.",
			},
			"sasl_aws_secret_key": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_SECRET_ACCESS_KEY", nil),
				Description: "The AWS secret key.",
			},
			"sasl_aws_token": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_SESSION_TOKEN", nil),
				Description: "The AWS session token. Only required if you are using temporary security credentials.",
			},
			"sasl_aws_creds_debug": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_CREDS_DEBUG", "false"),
				Description: "Set this to true to turn AWS credentials debug.",
			},
			"sasl_username": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_USERNAME", nil),
				Description: "Username for SASL authentication.",
			},
			"sasl_password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_PASSWORD", nil),
				Description: "Password for SASL authentication.",
			},
			"sasl_token_url": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_TOKEN_URL", nil),
				Description: "The url to retrieve oauth2 tokens from, when using sasl mechanism oauthbearer",
			},
			"sasl_mechanism": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_MECHANISM", "plain"),
				Description: "SASL mechanism, can be plain, scram-sha512, scram-sha256, aws-iam",
			},
			"skip_tls_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SKIP_VERIFY", "false"),
				Description: "Set this to true only if the target Kafka server is an insecure development instance.",
			},
			"tls_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_ENABLE_TLS", "true"),
				Description: "Enable communication with the Kafka Cluster over TLS.",
			},
			"timeout": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     120,
				Description: "Timeout in seconds",
			},
		},

		ConfigureFunc: providerConfigure,
		ResourcesMap: map[string]*schema.Resource{
			"kafka_topic":                 kafkaTopicResource(),
			"kafka_acl":                   kafkaACLResource(),
			"kafka_quota":                 kafkaQuotaResource(),
			"kafka_user_scram_credential": kafkaUserScramCredentialResource(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"kafka_topic": kafkaTopicDataSource(),
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	brokers := dTos("bootstrap_servers", d)

	log.Printf("[TRACE] configuring provider with brokers @ %v", brokers)

	saslMechanism := d.Get("sasl_mechanism").(string)
	switch saslMechanism {
	case "scram-sha512", "scram-sha256", "aws-iam", "oauthbearer", "plain":
	default:
		return nil, fmt.Errorf("[ERROR] Invalid sasl mechanism \"%s\": can only be \"scram-sha256\", \"scram-sha512\", \"aws-iam\", \"oauthbearer\" or \"plain\"", saslMechanism)
	}

	config := &Config{
		BootstrapServers:                       brokers,
		CACert:                                 d.Get("ca_cert").(string),
		ClientCert:                             d.Get("client_cert").(string),
		ClientCertKey:                          d.Get("client_key").(string),
		ClientCertKeyPassphrase:                d.Get("client_key_passphrase").(string),
		KafkaVersion:                           d.Get("kafka_version").(string),
		SkipTLSVerify:                          d.Get("skip_tls_verify").(bool),
		SASLAWSRegion:                          d.Get("sasl_aws_region").(string),
		SASLAWSContainerAuthorizationTokenFile: d.Get("sasl_aws_container_authorization_token_file").(string),
		SASLAWSContainerCredentialsFullUri:     d.Get("sasl_aws_container_credentials_full_uri").(string),
		SASLUsername:                           d.Get("sasl_username").(string),
		SASLPassword:                           d.Get("sasl_password").(string),
		SASLTokenUrl:                           d.Get("sasl_token_url").(string),
		SASLAWSRoleArn:                         d.Get("sasl_aws_role_arn").(string),
		SASLAWSExternalId:                      d.Get("sasl_aws_external_id").(string),
		SASLAWSProfile:                         d.Get("sasl_aws_profile").(string),
		SASLAWSAccessKey:                       d.Get("sasl_aws_access_key").(string),
		SASLAWSSecretKey:                       d.Get("sasl_aws_secret_key").(string),
		SASLAWSToken:                           d.Get("sasl_aws_token").(string),
		SASLAWSCredsDebug:                      d.Get("sasl_aws_creds_debug").(bool),
		SASLMechanism:                          saslMechanism,
		TLSEnabled:                             d.Get("tls_enabled").(bool),
		Timeout:                                d.Get("timeout").(int),
	}

	if config.CACert == "" {
		config.CACert = d.Get("ca_cert_file").(string)
	}
	if config.ClientCert == "" {
		config.ClientCert = d.Get("client_cert_file").(string)
	}
	if config.ClientCertKey == "" {
		config.ClientCertKey = d.Get("client_key_file").(string)
	}

	log.Printf("[TRACE] Config @ %v", config.copyWithMaskedSensitiveValues())

	return &LazyClient{
		Config: config,
	}, nil
}

func dTos(key string, d *schema.ResourceData) *[]string {
	var r *[]string

	if v, ok := d.GetOk(key); ok {
		if v == nil {
			return r
		}
		vI := v.([]interface{})
		b := make([]string, len(vI))

		for i, vv := range vI {
			if vv == nil {
				log.Printf("[DEBUG] %d %v was nil", i, vv)
				continue
			}
			b[i] = vv.(string)
		}
		r = &b
	}

	return r
}
