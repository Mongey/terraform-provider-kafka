package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const defaultIterations int32 = 4096

func validatePasswordFields(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
	password := d.Get("password").(string)
	passwordWo := d.Get("password_wo").(string)

	if password == "" && passwordWo == "" {
		return fmt.Errorf("either 'password' or 'password_wo' must be specified")
	}

	return nil
}

func kafkaUserScramCredentialResource() *schema.Resource {
	//lintignore:R011
	return &schema.Resource{
		CreateContext: userScramCredentialCreate,
		ReadContext:   userScramCredentialRead,
		UpdateContext: userScramCredentialUpdate,
		DeleteContext: userScramCredentialDelete,
		Importer: &schema.ResourceImporter{
			StateContext: importSCRAM,
		},
		CustomizeDiff: validatePasswordFields,
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
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      false,
				ValidateFunc:  validation.StringIsNotWhiteSpace,
				Description:   "The password of the credential (deprecated, use password_wo instead)",
				Sensitive:     true,
				ConflictsWith: []string{"password_wo"},
			},
			"password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      false,
				ValidateFunc:  validation.StringIsNotWhiteSpace,
				Description:   "The write-only password of the credential",
				Sensitive:     true,
				WriteOnly:     true,
				ConflictsWith: []string{"password"},
			},
			"password_wo_version": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    false,
				Description: "Version identifier for the write-only password to track changes",
			},
		},
	}
}

func importSCRAM(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), "|")
	if len(parts) == 2 {
		// New format: username|scram_mechanism (for write-only passwords)
		errSet := errSetter{d: d}
		errSet.Set("username", parts[0])
		errSet.Set("scram_mechanism", parts[1])
		// For write-only import, password_wo and password_wo_version need to be set manually after import
		if errSet.err != nil {
			return nil, errSet.err
		}
	} else if len(parts) == 3 {
		// Legacy format: username|scram_mechanism|password (for backward compatibility)
		errSet := errSetter{d: d}
		errSet.Set("username", parts[0])
		errSet.Set("scram_mechanism", parts[1])
		errSet.Set("password", parts[2])
		if errSet.err != nil {
			return nil, errSet.err
		}
	} else {
		return nil, fmt.Errorf("failed importing resource; expected format is username|scram_mechanism (for write-only passwords) or username|scram_mechanism|password (legacy) - got %v segments instead of 2 or 3", len(parts))
	}

	return []*schema.ResourceData{d}, nil
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

	// Get password from either password or password_wo field
	var password string
	if pw := d.Get("password").(string); pw != "" {
		password = pw
	} else {
		password = d.Get("password_wo").(string)
	}

	return UserScramCredential{
		Name:       d.Get("username").(string),
		Mechanism:  mechanism,
		Iterations: int32(d.Get("scram_iterations").(int)),
		Password:   []byte(password),
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
