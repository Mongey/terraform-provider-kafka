package kafka

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
)

func kafkaACLResource() *schema.Resource {
	return &schema.Resource{
		Create: aclCreate,
		Read:   aclRead,
		Delete: aclDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		SchemaVersion: 1,
		MigrateState:  migrateKafkaAclState,
		Schema: map[string]*schema.Schema{
			"resource_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the resouce",
			},
			"resource_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"resource_pattern_type_filter": {
				Type:     schema.TypeString,
				Required: false,
				Optional: true,
				Default:  "Literal",
				ForceNew: true,
			},
			"acl_principal": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"acl_host": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"acl_operation": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"acl_permission_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func aclCreate(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*Client)
	a := aclInfo(d)

	log.Printf("[INFO] Creating ACL %s", a)
	err := c.CreateACL(a)

	if err != nil {
		log.Println("[ERROR] Failed to create ACL")
		return err
	}

	d.SetId(a.String())

	return nil
}

func aclDelete(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*Client)
	a := aclInfo(d)
	log.Printf("[INFO] Deleting ACL %s", a)
	return c.DeleteACL(a)
}

func aclRead(d *schema.ResourceData, meta interface{}) error {
	log.Println("[INFO] Reading ACL")
	c := meta.(*Client)
	a := aclInfo(d)
	log.Printf("[INFO] Reading ACL %s", a)

	currentAcls, err := c.ListACLs()
	if err != nil {
		return err
	}
	for _, c := range currentAcls {
		if c.ResourceName == a.Resource.Name {
			log.Printf("[INFO] Found ACL %v", c)
			return nil
		}

	}
	return nil
}

func aclInfo(d *schema.ResourceData) stringlyTypedACL {
	s := stringlyTypedACL{
		ACL: ACL{
			Principal:      d.Get("acl_principal").(string),
			Host:           d.Get("acl_host").(string),
			Operation:      d.Get("acl_operation").(string),
			PermissionType: d.Get("acl_permission_type").(string),
		},
		Resource: Resource{
			Type:              d.Get("resource_type").(string),
			Name:              d.Get("resource_name").(string),
			PatternTypeFilter: d.Get("resource_pattern_type_filter").(string),
		},
	}
	return s
}
