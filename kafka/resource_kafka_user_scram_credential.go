package kafka

import (
	"context"
	"log"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const defaultIterations int32 = 4096

func kafkaUserScramCredentialResource() *schema.Resource {
	//lintignore:R011
	return &schema.Resource{
		CreateContext: userScramCredentialCreate,
		ReadContext:   userScramCredentialRead,
		UpdateContext: userScramCredentialUpdate,
		DeleteContext: userScramCredentialDelete,
		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the credential",
			},
			"scram_mechanism": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateDiagFunc(validation.StringInSlice([]string{sarama.SASLTypeSCRAMSHA256, sarama.SASLTypeSCRAMSHA512}, false)),
				Description:      "The SCRAM mechanism used to generate the credential (SCRAM-SHA-256, SCRAM-SHA-512)",
			},
			"scram_iterations": {
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     false,
				Default:      defaultIterations,
				ValidateFunc: validation.IntAtLeast(4096),
				Description:  "The number of SCRAM iterations used when generating the credential",
			},
			"password": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     false,
				ValidateFunc: validation.StringIsNotWhiteSpace,
				Description:  "The password of the credential",
				Sensitive:    true,
			},
		},
	}
}

func userScramCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[INFO] Creating user scram credential")
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)

	err := c.UpsertUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to create user scram credential")
		return diag.FromErr(err)
	}

	d.SetId(userScramCredential.ID())
	return nil
}

func userScramCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Println("[INFO] Reading user scram credential")
	c := meta.(*LazyClient)
	username := d.Get("username").(string)
	mechanism := d.Get("scram_mechanism").(string)

	userScramCredential, err := c.DescribeUserScramCredential(username, mechanism)
	if err != nil {
		log.Printf("[ERROR] Error getting user scram credential %s from Kafka", err)
		_, ok := err.(UserScramCredentialMissingError)
		if ok {
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] Setting the state from Kafka %v", userScramCredential)

	errSet := errSetter{d: d}
	errSet.Set("username", userScramCredential.Name)
	errSet.Set("scram_mechanism", userScramCredential.Mechanism.String())
	errSet.Set("scram_iterations", userScramCredential.Iterations)

	if errSet.err != nil {
		return diag.FromErr(errSet.err)
	}

	return nil
}

func userScramCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[INFO] Updating user scram credential")
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)

	err := c.UpsertUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to update user scram credential")
		return diag.FromErr(err)
	}

	return nil
}

func userScramCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Printf("[INFO] Deleting user scram credential")
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)

	err := c.DeleteUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to delete user scram credential")
		return diag.FromErr(err)
	}

	return nil
}

func parseUserScramCredential(d *schema.ResourceData) UserScramCredential {
	scram_mechanism_string := d.Get("scram_mechanism").(string)
	mechanism := convertedScramMechanism(scram_mechanism_string)
	return UserScramCredential{
		Name:       d.Get("username").(string),
		Mechanism:  mechanism,
		Iterations: int32(d.Get("scram_iterations").(int)),
		Password:   []byte(d.Get("password").(string)),
	}
}

func convertedScramMechanism(scram_mechanism_string string) sarama.ScramMechanismType {
	switch scram_mechanism_string {
	case sarama.SCRAM_MECHANISM_SHA_256.String():
		return sarama.SCRAM_MECHANISM_SHA_256
	case sarama.SCRAM_MECHANISM_SHA_512.String():
		return sarama.SCRAM_MECHANISM_SHA_512
	default:
		return sarama.SCRAM_MECHANISM_UNKNOWN
	}
}
