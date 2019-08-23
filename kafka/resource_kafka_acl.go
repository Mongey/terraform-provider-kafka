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
	ACLnotFound := false

	for _, broker := range currentACLs {
		if broker.ResourceName != a.Resource.Name {
			continue
		}
		if len(broker.Acls) < 1 {
			break
		}
		ACLnotFound = true
		log.Printf("[INFO] Found (%d) ACL(s) for Resource %s: %+v.", len(broker.Acls), broker.ResourceName, broker)

		for _, acl := range broker.Acls {
			aclID := stringlyTypedACL{
				ACL: ACL{
					Principal:      acl.Principal,
					Host:           acl.Host,
					Operation:      ACLOperationToString(acl.Operation),
					PermissionType: ACLPermissionTypeToString(acl.PermissionType),
				},
				Resource: Resource{
					Type: ACLResouceToString(broker.ResourceType),
					Name: broker.ResourceName,
				},
			}
			if a.String() == aclID.String() {
				d.Set("acl_principal", acl.Principal)
				d.Set("acl_host", acl.Host)
				d.Set("acl_operation", acl.Operation)
				d.Set("acl_permission_type", acl.PermissionType)
				return nil
			}
		}
	}
	if ACLnotFound {
		d.SetId("")
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
			Type: d.Get("resource_type").(string),
			Name: d.Get("resource_name").(string),
		},
	}
	return s
}
