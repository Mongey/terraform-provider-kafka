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
				Default:     90,
				Description: "Timeout in seconds",
			},
		},

		ConfigureFunc: providerConfigure,
		ResourcesMap: map[string]*schema.Resource{
			"kafka_topic": kafkaTopicResource(),
		},
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	var brokers *[]string

	if brokersRaw, ok := d.GetOk("bootstrap_servers"); ok {
		brokerI := brokersRaw.([]interface{})
		log.Printf("[DEBUG] configuring provider with Brokers of size %d", len(brokerI))
		b := make([]string, len(brokerI))
		for i, v := range brokerI {
			b[i] = v.(string)
		}
		log.Printf("[DEBUG] b of size %d", len(b))
		brokers = &b
	} else {
		log.Printf("[ERROR] something wrong? %v , ", d.Get("timeout"))
		return nil, fmt.Errorf("brokers was not set")
	}

	log.Printf("[DEBUG] configuring provider with Brokers @ %v", brokers)
	timeout := d.Get("timeout").(int)

	config := &Config{
		BootstrapServers: brokers,
		CACertFile:       d.Get("ca_cert_file").(string),
		ClientCertFile:   d.Get("client_cert_file").(string),
		ClientCertKey:    d.Get("client_key_file").(string),
		SkipTLSVerify:    d.Get("skip_tls_verify").(bool),
		SASLUsername:     d.Get("sasl_username").(string),
		SASLPassword:     d.Get("sasl_password").(string),
		TLSEnabled:       d.Get("tls_enabled").(bool),
		Timeout:          timeout,
	}

	log.Printf("[DEBUG] Config @ %v", config)

	return NewClient(config)
}
