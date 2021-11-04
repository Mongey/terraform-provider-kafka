package kafka

import (
	"context"
	"log"
	"time"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

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
				Optional:         true,
				ForceNew:         false,
				Default: 		  sarama.SASLTypeSCRAMSHA512,
				ValidateDiagFunc: validateDiagFunc(validation.StringInSlice([]string{sarama.SASLTypeSCRAMSHA256, sarama.SASLTypeSCRAMSHA512}, false)),
				Description:      "The required mechanism of the scram (SCRAM_SHA_256, SCRAM_SHA_512)",
			},
			"scram_iterations": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    false,
				Default: 	 4096,
				ValidateFunc: validation.IntAtLeast(4096),
				Description: "The number of iterations used when creating the credential",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    false,
				ValidateFunc: validation.StringIsNotWhiteSpace,
				Description: "The password of the credential",
			},
		},
	}
}

func userScramCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)
	log.Printf("[INFO] Creating user scram credential %s", userScramCredential)

	err := c.UpsertUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to create user scram credential")
		return diag.FromErr(err)
	}

	d.SetId(userScramCredential.ID())
	if err := waitForUserScramCredentialRefresh(ctx, c, d.Id(), userScramCredential); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func userScramCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)
	log.Printf("[INFO] Updating user scram credential %s", userScramCredential)

	err := c.UpsertUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to update user scram credential")
		return diag.FromErr(err)
	}

	if err := waitForUserScramCredentialRefresh(ctx, c, d.Id(), userScramCredential); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func userScramCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	userScramCredential := parseUserScramCredential(d)
	log.Printf("[INFO] Deleting user scram credential %s", userScramCredential)

	err := c.DeleteUserScramCredential(userScramCredential)
	if err != nil {
		log.Println("[ERROR] Failed to delete user scram credential")
		return diag.FromErr(err)
	}

	return nil
}

func userScramCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Println("[INFO] Reading UserScramCredential")
	c := meta.(*LazyClient)

	username := d.Get("username").(string)
	log.Printf("[INFO] Reading user scram credential %s", username)

	userScramCredential, err := c.DescribeUserScramCredential(username)
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

	log.Printf("[INFO] Found user scram credential %s.", userScramCredential.ID())
	return nil
}

func parseUserScramCredential(d *schema.ResourceData) UserScramCredential {
	scram_mechanism_string := d.Get("scram_mechanism").(string)
	mechanism := convertScramMechanismStringToEnum(scram_mechanism_string)
	return UserScramCredential{
		Name: d.Get("username").(string),
		Mechanism: mechanism,
		Iterations: d.Get("scram_iterations").(int32),
		Password: d.Get("password").(string),
	}
}

func convertScramMechanismStringToEnum(scram_mechanism_string string) sarama.ScramMechanismType {
	switch scram_mechanism_string {
	case sarama.SASLTypeSCRAMSHA256:
		return sarama.SCRAM_MECHANISM_SHA_256
	case sarama.SASLTypeSCRAMSHA512:
		return sarama.SCRAM_MECHANISM_SHA_512
	default:
		return sarama.SCRAM_MECHANISM_UNKNOWN
	}
}


func waitForUserScramCredentialRefresh(ctx context.Context, client *LazyClient, username string, expected UserScramCredential) error {
	timeout := time.Duration(client.Config.Timeout) * time.Second
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"Upserting"},
		Target:       []string{"Ready"},
		Refresh:      userScramCredentialRefreshFunc(client, username, expected),
		Timeout:      timeout,
		Delay:        1 * time.Second,
		PollInterval: 2 * time.Second,
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return fmt.Errorf(
			"Error waiting for user scram credential (%s) to become ready: %s",
			username, err)
	}

	return nil
}

func userScramCredentialRefreshFunc(client *LazyClient, username string, expected UserScramCredential) resource.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		log.Printf("[DEBUG] waiting for user scram credential to update %s", username)
		actual, err := client.DescribeUserScramCredential(username)
		if err != nil {
			log.Printf("[ERROR] could not read user scram credential %s, %s", username, err)
			return *actual, "Error", err
		}

		return *actual, "Ready", nil
	}
}
