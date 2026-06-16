package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func validateCompositeEntityName(v interface{}, k string) ([]string, []error) {
	s, ok := v.(string)
	if !ok {
		return nil, []error{fmt.Errorf("%s: expected string, got %T", k, v)}
	}
	for _, ch := range []string{"|", ":"} {
		if strings.Contains(s, ch) {
			return nil, []error{fmt.Errorf("%s: entity name %q must not contain %q", k, s, ch)}
		}
	}
	if s == entityDefault {
		return nil, []error{fmt.Errorf("%s: entity name %q is reserved; omit the name attribute to target the entity-default", k, s)}
	}
	return nil, nil
}

func compositeEntityHash(v interface{}) int {
	m := v.(map[string]interface{})
	typ, _ := m["type"].(string)
	name, _ := m["name"].(string)
	return schema.HashString(typ + "|" + name)
}

func kafkaQuotaCompositeResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: compositeQuotaCreate,
		ReadContext:   compositeQuotaRead,
		DeleteContext: compositeQuotaDelete,
		Schema: map[string]*schema.Schema{
			"entity": {
				Type:        schema.TypeSet,
				Required:    true,
				ForceNew:    true,
				MinItems:    2,
				MaxItems:    2,
				Set:         compositeEntityHash,
				Description: "Exactly two entity blocks identifying the composite quota target. Order does not matter.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:             schema.TypeString,
							Required:         true,
							ForceNew:         true,
							ValidateDiagFunc: validateDiagFunc(validation.StringInSlice([]string{"client-id", "user"}, false)),
							Description:      "Entity type: client-id or user.",
						},
						"name": {
							Type:         schema.TypeString,
							Optional:     true,
							ForceNew:     true,
							ValidateFunc: validateCompositeEntityName,
							Description:  "Entity name. Omit to target the entity-default.",
						},
					},
				},
			},
			"config": {
				Type:        schema.TypeMap,
				Required:    true,
				ForceNew:    true,
				Description: "Quota configuration keys to float64 values. Valid keys: producer_byte_rate, consumer_byte_rate, request_percentage, controller_mutation_rate.",
				Elem:        &schema.Schema{Type: schema.TypeFloat},
			},
		},
	}
}

func compositeQuotaCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	quota, err := newCompositeQuota(d, false)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Creating composite quota %s", quota)

	if err := c.AlterCompositeQuota(quota); err != nil {
		log.Println("[ERROR] Failed to create composite quota")
		return diag.FromErr(err)
	}

	d.SetId(quota.ID())

	stateConf := &retry.StateChangeConf{
		Pending:      []string{"Pending"},
		Target:       []string{"Created"},
		Refresh:      compositeQuotaCreatedFunc(c, quota),
		Timeout:      time.Duration(c.Config.Timeout) * time.Second,
		Delay:        1 * time.Second,
		PollInterval: 2 * time.Second,
	}

	if _, err := stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(fmt.Errorf("error waiting for composite quota (%s) to be created: %s", quota.ID(), err))
	}

	return compositeQuotaRead(ctx, d, meta)
}

func compositeQuotaCreatedFunc(client *LazyClient, q CompositeQuota) retry.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		fq, err := client.DescribeCompositeQuota(q.Entities)
		switch e := err.(type) {
		case QuotaMissingError:
			return &CompositeQuota{}, "Pending", nil
		case nil:
			return fq, "Created", nil
		default:
			return fq, "Error", e
		}
	}
}

func compositeQuotaDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	c := meta.(*LazyClient)
	quota, err := newCompositeQuota(d, true)
	if err != nil {
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Deleting composite quota %s", quota)

	if err := c.AlterCompositeQuota(quota); err != nil {
		log.Println("[ERROR] Failed to delete composite quota")
		return diag.FromErr(err)
	}

	return nil
}

func compositeQuotaRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log.Println("[INFO] Reading composite quota")
	c := meta.(*LazyClient)

	entities := compositeSchemaToEntities(d)
	foundQuota, err := c.DescribeCompositeQuota(entities)
	if err != nil {
		log.Printf("[ERROR] Error reading composite quota from Kafka: %s", err)
		if _, ok := err.(QuotaMissingError); ok {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	configs := make(map[string]interface{}, len(foundQuota.Ops))
	for _, op := range foundQuota.Ops {
		configs[op.Key] = op.Value
	}

	entityList := make([]interface{}, len(foundQuota.Entities))
	for i, e := range foundQuota.Entities {
		entityList[i] = map[string]interface{}{"type": e.Type, "name": e.Name}
	}

	errSet := errSetter{d: d}
	errSet.Set("entity", entityList)
	errSet.Set("config", configs)
	if errSet.err != nil {
		return diag.FromErr(errSet.err)
	}

	log.Printf("[INFO] Found composite quota %s %+v.", foundQuota.ID(), foundQuota.Ops)
	return nil
}

func compositeSchemaToEntities(d *schema.ResourceData) []CompositeQuotaEntity {
	raw := d.Get("entity").(*schema.Set).List()
	entities := make([]CompositeQuotaEntity, 0, len(raw))
	for _, v := range raw {
		if v == nil {
			continue
		}
		m := v.(map[string]interface{})
		typ, _ := m["type"].(string)
		name, _ := m["name"].(string)
		entities = append(entities, CompositeQuotaEntity{Type: typ, Name: name})
	}
	return entities
}

func newCompositeQuota(d *schema.ResourceData, removeAll bool) (CompositeQuota, error) {
	config := d.Get("config").(map[string]interface{})
	ops, err := configOpsFromMap(config, removeAll)
	if err != nil {
		return CompositeQuota{}, err
	}
	return CompositeQuota{Entities: compositeSchemaToEntities(d), Ops: ops}, nil
}
