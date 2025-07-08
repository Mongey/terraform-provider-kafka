package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func kafkaACLResource() *schema.Resource {
	//lintignore:R011
	return &schema.Resource{
		CreateContext: aclCreate,
		ReadContext:   aclRead,
		DeleteContext: aclDelete,
		Importer: &schema.ResourceImporter{
			StateContext: importACL,
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
				Default:  "Literal",
				Optional: true,
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

func aclCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c := meta.(*LazyClient)
	a := aclInfo(d)

	log.Printf("[INFO] Creating ACL %s", a)
	err := c.CreateACL(a)

	if err != nil {
		log.Println("[ERROR] Failed to create ACL")
		return diag.FromErr(err)
	}

	d.SetId(a.String())

	// Wait for ACL to be visible in Kafka before returning
	// This handles eventual consistency and ensures the ACL is actually created
	log.Printf("[INFO] Waiting for ACL %s to be visible in Kafka", a)
	err = waitForACLToBeVisible(ctx, c, a)
	if err != nil {
		log.Printf("[ERROR] ACL created but not visible: %v", err)
		return diag.FromErr(err)
	}

	return nil
}

func aclDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	c := meta.(*LazyClient)
	a := aclInfo(d)
	log.Printf("[INFO] Deleting ACL %s", a)

	err := c.DeleteACL(a)
	if err != nil {
		return diag.FromErr(err)
	}

	// Wait for ACL to be removed from Kafka before returning
	// This handles eventual consistency and ensures the ACL is actually deleted
	log.Printf("[INFO] Waiting for ACL %s to be removed from Kafka", a)
	err = waitForACLToBeDeleted(ctx, c, a)
	if err != nil {
		log.Printf("[ERROR] ACL deletion requested but still visible: %v", err)
		return diag.FromErr(err)
	}

	return nil
}

func aclRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	log.Println("[INFO] Reading ACL")
	c := meta.(*LazyClient)
	a := aclInfo(d)
	log.Printf("[INFO] Reading ACL %s", a)

	currentACLs, err := c.ListACLs()
	if err != nil {
		return diag.FromErr(err)
	}

	for _, foundACLs := range currentACLs {
		// find only ACLs where ResourceName matches
		if foundACLs.ResourceName != a.Resource.Name {
			continue
		}
		if len(foundACLs.Acls) < 1 {
			continue
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
					Type:              ACLResourceToString(foundACLs.ResourceType),
					Name:              foundACLs.ResourceName,
					PatternTypeFilter: foundACLs.ResourcePatternType.String(),
				},
			}

			// Found the ACL, so no need to remove it from state
			if a.String() == aclID.String() {
				return nil
			}
		}
	}

	// If we get here, the ACL was not found
	log.Printf("[INFO] Did not find ACL %s", a.String())
	d.SetId("")

	return nil
}

func importACL(ctx context.Context, d *schema.ResourceData, m any) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), "|")
	if len(parts) == 7 {
		errSet := errSetter{d: d}
		errSet.Set("acl_principal", parts[0])
		errSet.Set("acl_host", parts[1])
		errSet.Set("acl_operation", parts[2])
		errSet.Set("acl_permission_type", parts[3])
		errSet.Set("resource_type", parts[4])
		errSet.Set("resource_name", parts[5])
		errSet.Set("resource_pattern_type_filter", parts[6])
		if errSet.err != nil {
			return nil, errSet.err
		}
	} else {
		return nil, fmt.Errorf("failed importing resource; expected format is acl_principal|acl_host|acl_operation|acl_permission_type|resource_type|resource_name|resource_pattern_type_filter - got %v segments instead of 7", len(parts))
	}

	return []*schema.ResourceData{d}, nil
}

type errSetter struct {
	err error
	d   *schema.ResourceData
}

func (es *errSetter) Set(key string, value any) {
	if es.err != nil {
		return
	}
	//lintignore:R001
	es.err = es.d.Set(key, value)
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

// waitForACLToBeVisible waits for an ACL to be visible in Kafka after creation
// This handles eventual consistency issues with Kafka ACL propagation
func waitForACLToBeVisible(ctx context.Context, c *LazyClient, expectedACL StringlyTypedACL) error {
	maxRetries := 10
	retryInterval := 200 * time.Millisecond

	for i := range maxRetries {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Invalidate cache to ensure we get fresh data
		err := c.InvalidateACLCache()
		if err != nil {
			return fmt.Errorf("failed to invalidate ACL cache: %w", err)
		}

		// List all ACLs
		acls, err := c.ListACLs()
		if err != nil {
			return fmt.Errorf("failed to list ACLs: %w", err)
		}

		// Check if our ACL exists
		for _, foundACLs := range acls {
			if foundACLs.ResourceName != expectedACL.Resource.Name {
				continue
			}

			for _, acl := range foundACLs.Acls {
				foundACL := StringlyTypedACL{
					ACL: ACL{
						Principal:      acl.Principal,
						Host:           acl.Host,
						Operation:      ACLOperationToString(acl.Operation),
						PermissionType: ACLPermissionTypeToString(acl.PermissionType),
					},
					Resource: Resource{
						Type:              ACLResourceToString(foundACLs.ResourceType),
						Name:              foundACLs.ResourceName,
						PatternTypeFilter: foundACLs.ResourcePatternType.String(),
					},
				}

				// Check for exact match
				if expectedACL.String() == foundACL.String() {
					log.Printf("[INFO] ACL %s is now visible in Kafka (attempt %d)", expectedACL, i+1)
					return nil
				}
			}
		}

		// If not found and not the last attempt, wait before retrying
		if i < maxRetries-1 {
			log.Printf("[DEBUG] ACL %s not yet visible, retrying in %v (attempt %d/%d)", expectedACL, retryInterval, i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("ACL %s was not visible in Kafka after %d attempts over %v", expectedACL, maxRetries, time.Duration(maxRetries)*retryInterval)
}

// waitForACLToBeDeleted waits for an ACL to be removed from Kafka after deletion
// This handles eventual consistency issues with Kafka ACL propagation
func waitForACLToBeDeleted(ctx context.Context, c *LazyClient, deletedACL StringlyTypedACL) error {
	maxRetries := 10
	retryInterval := 200 * time.Millisecond

	for i := range maxRetries {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Invalidate cache to ensure we get fresh data
		err := c.InvalidateACLCache()
		if err != nil {
			return fmt.Errorf("failed to invalidate ACL cache: %w", err)
		}

		// List all ACLs
		acls, err := c.ListACLs()
		if err != nil {
			return fmt.Errorf("failed to list ACLs: %w", err)
		}

		// Check if our ACL still exists
		found := false
		for _, foundACLs := range acls {
			if foundACLs.ResourceName != deletedACL.Resource.Name {
				continue
			}

			for _, acl := range foundACLs.Acls {
				foundACL := StringlyTypedACL{
					ACL: ACL{
						Principal:      acl.Principal,
						Host:           acl.Host,
						Operation:      ACLOperationToString(acl.Operation),
						PermissionType: ACLPermissionTypeToString(acl.PermissionType),
					},
					Resource: Resource{
						Type:              ACLResourceToString(foundACLs.ResourceType),
						Name:              foundACLs.ResourceName,
						PatternTypeFilter: foundACLs.ResourcePatternType.String(),
					},
				}

				// Check for exact match
				if deletedACL.String() == foundACL.String() {
					found = true
					break
				}
			}

			if found {
				break
			}
		}

		// If not found, the ACL has been successfully deleted
		if !found {
			log.Printf("[INFO] ACL %s has been removed from Kafka (attempt %d)", deletedACL, i+1)
			return nil
		}

		// If still found and not the last attempt, wait before retrying
		if i < maxRetries-1 {
			log.Printf("[DEBUG] ACL %s still visible, retrying in %v (attempt %d/%d)", deletedACL, retryInterval, i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("ACL %s was still visible in Kafka after %d attempts over %v", deletedACL, maxRetries, time.Duration(maxRetries)*retryInterval)
}
