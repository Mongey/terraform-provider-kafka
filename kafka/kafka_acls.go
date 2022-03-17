package kafka

import (
	"fmt"
	"log"
	"strings"

	"github.com/Shopify/sarama"
)

type ACL struct {
	Principal      string `json:"principal"`
	Host           string `json:"host"`
	Operation      string `json:"operation"`
	PermissionType string `json:"permission_type"`
}

type Resource struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	PatternTypeFilter string `json:"pattern_type_filter"`
}

type StringlyTypedACL struct {
	ACL      ACL `json:"acl"`
	Resource `json:"resource"`
}

func (a StringlyTypedACL) String() string {
	return strings.Join([]string{a.ACL.Principal, a.ACL.Host, a.ACL.Operation, a.ACL.PermissionType, a.Resource.Type, a.Resource.Name, a.Resource.PatternTypeFilter}, "|")
}

func tfToAclCreation(s StringlyTypedACL) (*sarama.AclCreation, error) {
	acl := &sarama.AclCreation{}

	op := stringToOperation(s.ACL.Operation)
	if op == unknownConversion {
		return acl, fmt.Errorf("Unknown operation: %s", s.ACL.Operation)
	}
	pType := stringToAclPermissionType(s.ACL.PermissionType)
	if pType == unknownConversion {
		return acl, fmt.Errorf("Unknown permission type: %s", s.ACL.PermissionType)
	}
	rType := stringToACLResource(s.Resource.Type)
	if rType == unknownConversion {
		return acl, fmt.Errorf("Unknown resource type: %s", s.Resource.Type)
	}
	patternType := stringToACLPrefix(s.Resource.PatternTypeFilter)
	if patternType == unknownConversion {
		return acl, fmt.Errorf("Unknown pattern type filter: '%s'", s.Resource.PatternTypeFilter)
	}

	acl.Acl = sarama.Acl{
		Principal:      s.ACL.Principal,
		Host:           s.ACL.Host,
		Operation:      op,
		PermissionType: pType,
	}
	acl.Resource = sarama.Resource{
		ResourceType:        rType,
		ResourceName:        s.Resource.Name,
		ResourcePatternType: patternType,
	}

	return acl, nil
}

const unknownConversion = -1

func tfToAclFilter(s StringlyTypedACL) (sarama.AclFilter, error) {
	f := sarama.AclFilter{
		Principal:    &s.ACL.Principal,
		Host:         &s.ACL.Host,
		ResourceName: &s.Resource.Name,
	}

	op := stringToOperation(s.ACL.Operation)
	if op == unknownConversion {
		return f, fmt.Errorf("Unknown operation: %s", s.ACL.Operation)
	}
	f.Operation = op

	pType := stringToAclPermissionType(s.ACL.PermissionType)
	if pType == unknownConversion {
		return f, fmt.Errorf("Unknown permission type: %s", s.ACL.PermissionType)
	}
	f.PermissionType = pType

	rType := stringToACLResource(s.Resource.Type)
	if rType == unknownConversion {
		return f, fmt.Errorf("Unknown resource type: %s", s.Resource.Type)
	}
	f.ResourceType = rType

	patternType := stringToACLPrefix(s.Resource.PatternTypeFilter)
	if patternType == unknownConversion {
		return f, fmt.Errorf("Unknown pattern type filter: '%s'", s.Resource.PatternTypeFilter)
	}
	f.ResourcePatternTypeFilter = patternType

	return f, nil
}

func stringToACLPrefix(s string) sarama.AclResourcePatternType {
	switch s {
	case "Any":
		return sarama.AclPatternAny
	case "Match":
		return sarama.AclPatternMatch
	case "Literal":
		return sarama.AclPatternLiteral
	case "Prefixed":
		return sarama.AclPatternPrefixed
	}
	return unknownConversion
}

func resourcePatternToString(p sarama.AclResourcePatternType) string {
	switch p {
	case sarama.AclPatternAny:
		return "Any"
	case sarama.AclPatternMatch:
		return "Match"
	case sarama.AclPatternLiteral:
		return "Literal"
	case sarama.AclPatternPrefixed:
		return "Prefixed"
	}
	return "unknownConversion"
}

func (c *Client) DeleteACL(s StringlyTypedACL) error {
	log.Printf("[INFO] Deleting ACL %v", s)
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}
	aclsBeforeDelete, err := c.ListACLs()
	if err != nil {
		return fmt.Errorf("Unable to list acls before deleting -- can't be sure we're doing the right thing: %s", err)
	}

	log.Printf("[INFO] Acls before deletion: %d", len(aclsBeforeDelete))
	for _, acl := range aclsBeforeDelete {
		log.Printf("[DEBUG] ACL: %v", acl)
	}

	filter, err := tfToAclFilter(s)
	if err != nil {
		return err
	}

	req := &sarama.DeleteAclsRequest{
		Version: int(c.getDeleteAclsRequestAPIVersion()),
		Filters: []*sarama.AclFilter{&filter},
	}

	res, err := broker.DeleteAcls(req)
	if err != nil {
		return err
	}

	for _, r := range res.FilterResponses {
		if r.Err != sarama.ErrNoError {
			return r.Err
		}
	}
	return nil
}

func (c *Client) CreateACL(s StringlyTypedACL) error {
	log.Printf("[DEBUG] Creating ACL %s", s)
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}

	ac, err := tfToAclCreation(s)
	if err != nil {
		return err
	}
	req := &sarama.CreateAclsRequest{
		Version:      c.getCreateAclsRequestAPIVersion(),
		AclCreations: []*sarama.AclCreation{ac},
	}

	res, err := broker.CreateAcls(req)
	if err != nil {
		return err
	}

	for _, r := range res.AclCreationResponses {
		if r.Err != sarama.ErrNoError {
			return r.Err
		}
	}
	log.Printf("[DEBUG] Created ACL %s", s)

	return nil
}

func stringToACLResource(in string) sarama.AclResourceType {
	switch in {
	case "Unknown":
		return sarama.AclResourceUnknown
	case "Any":
		return sarama.AclResourceAny
	case "Topic":
		return sarama.AclResourceTopic
	case "Group":
		return sarama.AclResourceGroup
	case "Cluster":
		return sarama.AclResourceCluster
	case "TransactionalID":
		return sarama.AclResourceTransactionalID
	}
	return unknownConversion
}

func ACLResourceToString(in sarama.AclResourceType) string {
	switch in {
	case sarama.AclResourceUnknown:
		return "Unknown"
	case sarama.AclResourceAny:
		return "Any"
	case sarama.AclResourceTopic:
		return "Topic"
	case sarama.AclResourceGroup:
		return "Group"
	case sarama.AclResourceCluster:
		return "Cluster"
	case sarama.AclResourceTransactionalID:
		return "TransactionalID"
	}
	return "unknownConversion"
}

func stringToOperation(in string) sarama.AclOperation {
	switch in {
	case "Unknown":
		return sarama.AclOperationUnknown
	case "Any":
		return sarama.AclOperationAny
	case "All":
		return sarama.AclOperationAll
	case "Read":
		return sarama.AclOperationRead
	case "Write":
		return sarama.AclOperationWrite
	case "Create":
		return sarama.AclOperationCreate
	case "Delete":
		return sarama.AclOperationDelete
	case "Alter":
		return sarama.AclOperationAlter
	case "Describe":
		return sarama.AclOperationDescribe
	case "ClusterAction":
		return sarama.AclOperationClusterAction
	case "DescribeConfigs":
		return sarama.AclOperationDescribeConfigs
	case "AlterConfigs":
		return sarama.AclOperationAlterConfigs
	case "IdempotentWrite":
		return sarama.AclOperationIdempotentWrite
	}
	return unknownConversion
}

// ACLOperationToString converts sarama.AclOperations to a String representation
func ACLOperationToString(in sarama.AclOperation) string {
	switch in {
	case sarama.AclOperationUnknown:
		return "Unknown"
	case sarama.AclOperationAny:
		return "Any"
	case sarama.AclOperationAll:
		return "All"
	case sarama.AclOperationRead:
		return "Read"
	case sarama.AclOperationWrite:
		return "Write"
	case sarama.AclOperationCreate:
		return "Create"
	case sarama.AclOperationDelete:
		return "Delete"
	case sarama.AclOperationAlter:
		return "Alter"
	case sarama.AclOperationDescribe:
		return "Describe"
	case sarama.AclOperationClusterAction:
		return "ClusterAction"
	case sarama.AclOperationDescribeConfigs:
		return "DescribeConfigs"
	case sarama.AclOperationAlterConfigs:
		return "AlterConfigs"
	case sarama.AclOperationIdempotentWrite:
		return "IdempotentWrite"
	}
	return "unknownConversion"
}

func stringToAclPermissionType(in string) sarama.AclPermissionType {
	switch in {
	case "Unknown":
		return sarama.AclPermissionUnknown
	case "Any":
		return sarama.AclPermissionAny
	case "Deny":
		return sarama.AclPermissionDeny
	case "Allow":
		return sarama.AclPermissionAllow
	}
	return unknownConversion
}

// ACLPermissionTypeToString converts sarama.AclPermissionTypes to Strings
func ACLPermissionTypeToString(in sarama.AclPermissionType) string {
	switch in {
	case sarama.AclPermissionUnknown:
		return "Unknown"
	case sarama.AclPermissionAny:
		return "Any"
	case sarama.AclPermissionDeny:
		return "Deny"
	case sarama.AclPermissionAllow:
		return "Allow"
	}
	return "unknownConversion"
}

// DescribeACLs get ResourceAcls for a specific resource
func (c *Client) DescribeACLs(s StringlyTypedACL) ([]*sarama.ResourceAcls, error) {
	aclFilter, err := tfToAclFilter(s)
	if err != nil {
		return nil, err
	}

	broker, err := c.client.Controller()
	if err != nil {
		return nil, err
	}
	err = c.client.RefreshMetadata()
	if err != nil {
		return nil, err
	}

	r := &sarama.DescribeAclsRequest{
		AclFilter: aclFilter,
	}

	aclsR, err := broker.DescribeAcls(r)
	if err != nil {
		return nil, err
	}

	if err == nil {
		if aclsR.Err != sarama.ErrNoError {
			return nil, fmt.Errorf("%s", aclsR.Err)
		}
	}

	return aclsR.ResourceAcls, err
}

func (c *Client) ListACLs() ([]*sarama.ResourceAcls, error) {
	log.Printf("[INFO] Listing all ACLS")
	broker, err := c.client.Controller()
	if err != nil {
		return nil, err
	}

	allResources := []*sarama.DescribeAclsRequest{
		&sarama.DescribeAclsRequest{
			Version: int(c.getDescribeAclsRequestAPIVersion()),
			AclFilter: sarama.AclFilter{
				ResourceType:              sarama.AclResourceTopic,
				ResourcePatternTypeFilter: sarama.AclPatternAny,
				PermissionType:            sarama.AclPermissionAny,
				Operation:                 sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			Version: int(c.getDescribeAclsRequestAPIVersion()),
			AclFilter: sarama.AclFilter{
				ResourceType:              sarama.AclResourceGroup,
				ResourcePatternTypeFilter: sarama.AclPatternAny,
				PermissionType:            sarama.AclPermissionAny,
				Operation:                 sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			Version: int(c.getDescribeAclsRequestAPIVersion()),
			AclFilter: sarama.AclFilter{
				ResourceType:              sarama.AclResourceCluster,
				ResourcePatternTypeFilter: sarama.AclPatternAny,
				PermissionType:            sarama.AclPermissionAny,
				Operation:                 sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			Version: int(c.getDescribeAclsRequestAPIVersion()),
			AclFilter: sarama.AclFilter{
				ResourceType:              sarama.AclResourceTransactionalID,
				ResourcePatternTypeFilter: sarama.AclPatternAny,
				PermissionType:            sarama.AclPermissionAny,
				Operation:                 sarama.AclOperationAny,
			},
		},
	}
	res := []*sarama.ResourceAcls{}

	log.Printf("[TRACE] Asking Kafka for all the resources")
	for _, r := range allResources {
		log.Printf("[TRACE] Describe Acl Requst %v", r)
		aclsR, err := broker.DescribeAcls(r)
		if err != nil {
			return nil, err
		}

		log.Printf("[TRACE] ThrottleTime: %d", aclsR.ThrottleTime)

		if err == nil {
			if aclsR.Err != sarama.ErrNoError {
				return nil, fmt.Errorf("%s", aclsR.Err)
			}
		}

		res = append(res, aclsR.ResourceAcls...)
	}

	return res, err
}
