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
	ACL      ACL      `json:"acl"`
	Resource Resource `json:"resource"`
}

func (a StringlyTypedACL) String() string {
	return strings.Join([]string{a.ACL.Principal, a.ACL.Host, a.ACL.Operation, a.ACL.PermissionType, a.Resource.Type, a.Resource.Name, a.Resource.PatternTypeFilter}, "|")
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

func (s StringlyTypedACL) AclCreation() (*sarama.AclCreation, error) {
	acl := &sarama.AclCreation{}

	op := stringToOperation(s.ACL.Operation)
	if op == unknownConversion {
		return acl, fmt.Errorf("Unknown operation: '%s'", s.ACL.Operation)
	}
	permissionType := stringToAclPermissionType(s.ACL.PermissionType)
	if permissionType == unknownConversion {
		return acl, fmt.Errorf("Unknown permission type: '%s'", s.ACL.PermissionType)
	}
	rType := stringToACLResouce(s.Resource.Type)
	if rType == unknownConversion {
		return acl, fmt.Errorf("Unknown resource type: '%s'", s.Resource.Type)
	}

	patternType := stringToACLPrefix(s.Resource.PatternTypeFilter)
	if patternType == unknownConversion {
		return acl, fmt.Errorf("Unknown pattern type filter: '%s'", s.Resource.PatternTypeFilter)
	}

	acl.Acl = sarama.Acl{
		Principal:      s.ACL.Principal,
		Host:           s.ACL.Host,
		Operation:      op,
		PermissionType: permissionType,
	}
	acl.Resource = sarama.Resource{
		ResourceType:       rType,
		ResourceName:       s.Resource.Name,
		ResoucePatternType: patternType,
	}

	return acl, nil
}

const unknownConversion = -1

func (s StringlyTypedACL) AclFilter() (sarama.AclFilter, error) {
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

	rType := stringToACLResouce(s.Resource.Type)
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

func (c *Client) DeleteACL(s StringlyTypedACL) error {
	broker, err := c.availableBroker()
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

	filter, err := s.AclFilter()
	if err != nil {
		return err
	}

	req := &sarama.DeleteAclsRequest{
		Version: c.aclVersion(),
		Filters: []*sarama.AclFilter{&filter},
	}

	log.Printf("[INFO] Deleting ACL %v\n", s)

	res, err := broker.DeleteAcls(req)
	if err != nil {
		return err
	}

	matchingAclCount := 0
	for _, r := range res.FilterResponses {
		log.Printf("Got %d matching Acls", len(r.MatchingAcls))
		matchingAclCount += len(r.MatchingAcls)
		if r.Err != sarama.ErrNoError {
			return r.Err
		}
	}

	if matchingAclCount == 0 {
		return fmt.Errorf("There were no acls matching this filter")
	}

	return nil
}

func stringToACLResouce(in string) sarama.AclResourceType {
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
