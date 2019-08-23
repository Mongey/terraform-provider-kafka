package kafka

import (
	"fmt"
	"log"
	"strings"

	"github.com/Shopify/sarama"
)

type ACL struct {
	Principal      string
	Host           string
	Operation      string
	PermissionType string
}

type Resource struct {
	Type string
	Name string
}

type stringlyTypedACL struct {
	ACL ACL
	Resource
}

func (a stringlyTypedACL) String() string {
	return strings.Join([]string{a.ACL.Principal, a.ACL.Host, a.ACL.Operation, a.ACL.PermissionType, a.Resource.Type, a.Resource.Name}, "|")
}

func tfToAclCreation(s stringlyTypedACL) (*sarama.AclCreation, error) {
	acl := &sarama.AclCreation{}

	op := stringToOperation(s.ACL.Operation)
	if op == unknownConversion {
		return acl, fmt.Errorf("Unknown operation: %s", s.ACL.Operation)
	}
	pType := stringToAclPermissionType(s.ACL.PermissionType)
	if pType == unknownConversion {
		return acl, fmt.Errorf("Unknown permission type: %s", s.ACL.PermissionType)
	}
	rType := stringToACLResouce(s.Resource.Type)
	if rType == unknownConversion {
		return acl, fmt.Errorf("Unknown resource type: %s", s.Resource.Type)
	}

	acl.Acl = sarama.Acl{
		Principal:      s.ACL.Principal,
		Host:           s.ACL.Host,
		Operation:      op,
		PermissionType: pType,
	}
	acl.Resource = sarama.Resource{
		ResourceType: rType,
		ResourceName: s.Resource.Name,
	}

	return acl, nil
}

const unknownConversion = -1

func tfToAclFilter(s stringlyTypedACL) (sarama.AclFilter, error) {
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

	return f, nil
}

func (c *Client) DeleteACL(s stringlyTypedACL) error {
	broker, err := c.availableBroker()
	if err != nil {
		return err
	}
	filter, err := tfToAclFilter(s)
	if err != nil {
		return err
	}

	req := &sarama.DeleteAclsRequest{
		Filters: []*sarama.AclFilter{&filter},
	}
	log.Printf("[INFO] Deleting ACL %v\n", s)

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

func (c *Client) CreateACL(s stringlyTypedACL) error {
	broker, err := c.availableBroker()
	if err != nil {
		return err
	}

	ac, err := tfToAclCreation(s)
	if err != nil {
		return err
	}
	req := &sarama.CreateAclsRequest{
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

// ACLResouceToString converts sarama.AclResourceTypes to Strings
func ACLResouceToString(in sarama.AclResourceType) string {
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

func (c *Client) ListACLs() ([]*sarama.ResourceAcls, error) {
	broker, err := c.availableBroker()
	if err != nil {
		return nil, err
	}
	err = c.client.RefreshMetadata()
	if err != nil {
		return nil, err
	}
	allResources := []*sarama.DescribeAclsRequest{
		&sarama.DescribeAclsRequest{
			AclFilter: sarama.AclFilter{
				ResourceType:   sarama.AclResourceTopic,
				PermissionType: sarama.AclPermissionAny,
				Operation:      sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			AclFilter: sarama.AclFilter{
				ResourceType:   sarama.AclResourceGroup,
				PermissionType: sarama.AclPermissionAny,
				Operation:      sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			AclFilter: sarama.AclFilter{
				ResourceType:   sarama.AclResourceCluster,
				PermissionType: sarama.AclPermissionAny,
				Operation:      sarama.AclOperationAny,
			},
		},
		&sarama.DescribeAclsRequest{
			AclFilter: sarama.AclFilter{
				ResourceType:   sarama.AclResourceTransactionalID,
				PermissionType: sarama.AclPermissionAny,
				Operation:      sarama.AclOperationAny,
			},
		},
	}
	res := []*sarama.ResourceAcls{}

	for _, r := range allResources {
		aclsR, err := broker.DescribeAcls(r)
		if err != nil {
			return nil, err
		}

		if err == nil {
			if aclsR.Err != sarama.ErrNoError {
				return nil, fmt.Errorf("%s", aclsR.Err)
			}
		}

		for _, a := range aclsR.ResourceAcls {
			res = append(res, a)
		}
	}
	return res, err
}
