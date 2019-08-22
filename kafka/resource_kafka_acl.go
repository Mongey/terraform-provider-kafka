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
	log.Printf("[INFO] Deleteing ACL %s", a)
	return c.DeleteACL(a)
}

func aclRead(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*Client)
	a := aclInfo(d)

	currentACLs, err := c.ListACLs()
	if err != nil {
		return err
	}

	log.Printf("[INFO] current acls: %d", len(currentACLs))
	for _, c := range currentACLs {
		if c.ResourceName != a.Resource.Name {
			continue
		}
		log.Printf("[INFO] Found ACL %v (%d)", c, len(c.Acls))

		if len(c.Acls) != 1 {
			return nil
		}

		d.Set("acl_principal", c.Acls[0].Principal)
		d.Set("acl_host", c.Acls[0].Host)
		d.Set("acl_operation", c.Acls[0].Operation)
		d.Set("acl_permission_type", c.Acls[0].PermissionType)

		return nil
	}
	return nil

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
			Type: d.Get("resource_type").(string),
			Name: d.Get("resource_name").(string),
		},
	}
	return s
}
