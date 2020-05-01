package kafka

import (
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
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
				Description: "The name of the resource",
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
	c := meta.(*LazyClient)
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
	c := meta.(*LazyClient)
	a := aclInfo(d)
	log.Printf("[INFO] Deleting ACL %s", a)
	return c.DeleteACL(a)
}

func aclRead(d *schema.ResourceData, meta interface{}) error {
	log.Println("[INFO] Reading ACL")
	c := meta.(*LazyClient)
	a := aclInfo(d)
	log.Printf("[INFO] Reading ACL %s", a)

	currentACLs, err := c.ListACLs()
	if err != nil {
		return err
	}

	aclNotFound := true

	for _, foundACLs := range currentACLs {
		// find only ACLs where ResourceName matches
		if foundACLs.ResourceName != a.Resource.Name {
			continue
		}
		if len(foundACLs.Acls) < 1 {
			break
		}
		log.Printf("[INFO] Found (%d) ACL(s) for Resource %s: %+v.", len(foundACLs.Acls), foundACLs.ResourceName, foundACLs)

		for _, acl := range foundACLs.Acls {
			aclID := StringlyTypedACL{
				ACL: ACL{
					Principal:      acl.Principal,
					Host:           acl.Host,
					Operation:      ACLOperationToString(acl.Operation),
					PermissionType: ACLPermissionTypeToString(acl.PermissionType),
				},
				Resource: Resource{
					Type: ACLResourceToString(foundACLs.ResourceType),
					Name: foundACLs.ResourceName,
				},
			}
			// exact match
			if a.String() == aclID.String() {
				aclNotFound = false
				return nil
			}

			// partial match -> update state
			if a.ACL.Principal == aclID.ACL.Principal &&
				a.ACL.Operation == aclID.ACL.Operation {
				aclNotFound = false
				d.Set("acl_principal", acl.Principal)
				d.Set("acl_host", acl.Host)
				d.Set("acl_operation", acl.Operation)
				d.Set("acl_permission_type", acl.PermissionType)
				d.Set("resource_pattern_type_filter", foundACLs.ResourcePatternType)
			}
		}
	}
	if aclNotFound {
		log.Printf("[INFO] Did not find ACL %s", a.String())
		d.SetId("")
	}
	return nil
}

func aclInfo(d *schema.ResourceData) StringlyTypedACL {
	s := StringlyTypedACL{
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
