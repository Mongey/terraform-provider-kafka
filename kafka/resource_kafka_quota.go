package kafka

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func kafkaQuotaResource() *schema.Resource {
	//lintignore:R011
	return &schema.Resource{
		CreateContext: quotaCreate,
		ReadContext:   quotaRead,
		DeleteContext: quotaDelete,
		Schema: map[string]*schema.Schema{
			"entity_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the entity",
			},
			"entity_type": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateDiagFunc: validateDiagFunc(validation.StringInSlice([]string{"client-id", "user", "ip"}, false)),
				Description:      "The type of the entity (client-id, user, ip)",
			},
			"config": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Description: "A map of string k/v properties.",
				Elem:        schema.TypeFloat,
			},
		},
	}
}

func quotaCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	quota := newQuota(d, false)
	log.Printf("[INFO] Creating Quota %s", quota)

	err := c.AlterQuota(quota)
	if err != nil {
		log.Println("[ERROR] Failed to create Quota")
		return diag.FromErr(err)
	}

	stateConf := &retry.StateChangeConf{
		Pending:      []string{"Pending"},
		Target:       []string{"Created"},
		Refresh:      quotaCreatedFunc(c, quota),
		Timeout:      time.Duration((*c.Config()).Timeout) * time.Second,
		Delay:        1 * time.Second,
		PollInterval: 2 * time.Second,
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(fmt.Errorf("error waiting for quota (%s) to be created: %s", quota.ID(), err))
	}

	d.SetId(quota.ID())

	return nil
}

func quotaCreatedFunc(client *LazyClient, q Quota) retry.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		fq, err := client.DescribeQuota(q.EntityType, q.EntityName)
		switch e := err.(type) {
		case QuotaMissingError:
			return fq, "Pending", nil
		case nil:
			return fq, "Created", nil
		default:
			return fq, "Error", e
		}
	}
}

func quotaDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	quota := newQuota(d, true)
	log.Printf("[INFO] Deleting quota %s", quota)

	err := c.AlterQuota(quota)
	if err != nil {
		log.Println("[ERROR] Failed to delete Quota")
		if c.Config().ForceDelete {
			log.Printf("[WARN] Force deleting quota %s despite error: %s", quota, err)
			return nil
		}
		return diag.FromErr(err)
	}

	return nil
}

func quotaRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Println("[INFO] Reading Quota")
	c := meta.(*LazyClient)

	entityType := d.Get("entity_type").(string)
	entityName := d.Get("entity_name").(string)
	log.Printf("[INFO] Reading Quota %s", entityName)

	foundQuota, err := c.DescribeQuota(entityType, entityName)
	if err != nil {
		log.Printf("[ERROR] Error getting quota %s from Kafka", err)
		_, ok := err.(QuotaMissingError)
		if ok {
			d.SetId("")
			return nil
		}

		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] Setting the state from Kafka %v", foundQuota)
	configs := map[string]float64{}
	for _, op := range foundQuota.Ops {
		configs[op.Key] = op.Value
	}

	errSet := errSetter{d: d}
	errSet.Set("entity_name", foundQuota.EntityName)
	errSet.Set("entity_type", foundQuota.EntityType)
	errSet.Set("config", configs)
	if errSet.err != nil {
		return diag.FromErr(errSet.err)
	}

	log.Printf("[INFO] Found Quota %s %+v.", foundQuota.ID(), foundQuota.Ops)
	return nil
}

func newQuota(d *schema.ResourceData, removeAll bool) Quota {
	config := d.Get("config").(map[string]interface{})
	ops := []QuotaOp{}
	for key, value := range config {
		switch value := value.(type) {
		case float64:
			ops = append(ops, QuotaOp{
				Key:    key,
				Value:  value,
				Remove: removeAll,
			})
		}
	}

	return Quota{
		EntityType: d.Get("entity_type").(string),
		EntityName: d.Get("entity_name").(string),
		Ops:        ops,
	}
}
