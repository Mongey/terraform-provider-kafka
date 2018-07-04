package kafka

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"bootstrap_servers": {
				Type:        schema.TypeList,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Required:    true,
				Description: "A list of kafka brokers",
			},
			"ca_cert_file": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CA_CERT", ""),
				Description: "Path to a CA certificate file to validate the server's certificate.",
			},
			"client_cert_file": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_CERT", ""),
				Description: "Path to a file containing the client certificate.",
			},
			"client_key_file": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_CLIENT_KEY", ""),
				Description: "Path to a file containing the private key that the certificate was issued for.",
			},
			"sasl_username": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_USERNAME", ""),
				Description: "Username for SASL authentication.",
			},
			"sasl_password": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SASL_PASSWORD", ""),
				Description: "Password for SASL authentication.",
			},
			"skip_tls_verify": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_SKIP_VERIFY", ""),
				Description: "Set this to true only if the target Kafka server is an insecure development instance.",
			},
			"tls_enabled": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("KAFKA_ENABLE_TLS", ""),
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
			"kafka_topic": kafkaTopicResource(),
			"kafka_acl":   kafkaACLResource(),
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	brokers := dTos("bootstrap_servers", d)

	if brokers == nil {
		return nil, fmt.Errorf("bootstrap_servers was not set")
	}
	log.Printf("[DEBUG] configuring provider with Brokers @ %v", brokers)

	config := &Config{
		BootstrapServers: brokers,
		CACertFile:       d.Get("ca_cert_file").(string),
		ClientCertFile:   d.Get("client_cert_file").(string),
		ClientCertKey:    d.Get("client_key_file").(string),
		SkipTLSVerify:    d.Get("skip_tls_verify").(bool),
		SASLUsername:     d.Get("sasl_username").(string),
		SASLPassword:     d.Get("sasl_password").(string),
		TLSEnabled:       d.Get("tls_enabled").(bool),
		Timeout:          d.Get("timeout").(int),
	}

	log.Printf("[DEBUG] Config @ %v", config)

	return NewClient(config)
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
			log.Printf("[DEBUG] %d:Converting %v to string", i, vv)
			b[i] = vv.(string)
		}
		r = &b
	}

	return r
}
