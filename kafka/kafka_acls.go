package kafka

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/IBM/sarama"
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

func (c *Client) enqueueDeleteACL(broker *sarama.Broker, filter *sarama.AclFilter) error {
	c.aclDeletionQueue.mutex.Lock()
	log.Printf("[DEBUG] Enqueueing ACL Deletion %v", *filter)
	if c.aclDeletionQueue.timer != nil {
		c.aclDeletionQueue.timer.Stop()
	}
	c.aclDeletionQueue.filters = append(c.aclDeletionQueue.filters, filter)
	c.aclDeletionQueue.waitChans = append(c.aclDeletionQueue.waitChans, make(chan error))
	var waitChan = c.aclDeletionQueue.waitChans[len(c.aclDeletionQueue.waitChans)-1]

	c.aclDeletionQueue.timer = time.AfterFunc(c.aclDeletionQueue.after, func() {
		c.aclDeletionQueue.mutex.Lock()
		defer c.aclDeletionQueue.mutex.Unlock()
		log.Printf("[INFO] Deleting ACLs %v", c.aclDeletionQueue.filters)
		defer func() {
			c.aclDeletionQueue.timer = nil
			c.aclDeletionQueue.filters = nil
			c.aclDeletionQueue.waitChans = nil
		}()
		req := &sarama.DeleteAclsRequest{
			Version: int(c.getDeleteAclsRequestAPIVersion()),
			Filters: c.aclDeletionQueue.filters,
		}

		res, err := broker.DeleteAcls(req)
		if err != nil {
			for _, wc := range c.aclDeletionQueue.waitChans {
				wc <- err
			}
			return
		}

		if len(res.FilterResponses) != len(c.aclDeletionQueue.waitChans) {
			for _, ch := range c.aclDeletionQueue.waitChans {
				ch <- fmt.Errorf("unexpectedly got a different length (%d) of FilterResponses compared to queued requests (%d) - this shouldn't be possible", len(res.FilterResponses), len(c.aclDeletionQueue.waitChans))
			}
			return
		}

		c.InvalidateACLCache()
		for i, r := range res.FilterResponses {
			if r.Err != sarama.ErrNoError {
				c.aclDeletionQueue.waitChans[i] <- r.Err
			} else {
				c.aclDeletionQueue.waitChans[i] <- nil
			}
		}
	})

	c.aclDeletionQueue.mutex.Unlock()
	return <-waitChan
}

func (c *Client) DeleteACL(s StringlyTypedACL) error {
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}

	filter, err := tfToAclFilter(s)
	if err != nil {
		return err
	}

	err = c.enqueueDeleteACL(broker, &filter)

	if err != nil {
		return err
	}

	return nil
}

func (c *Client) enqueueCreateACL(broker *sarama.Broker, create *sarama.AclCreation) error {
	c.aclCreationQueue.mutex.Lock()
	if c.aclCreationQueue.timer != nil {
		c.aclCreationQueue.timer.Stop()
	}
	log.Printf("[DEBUG] Enqueueing ACL Creation %v", create.Acl)
	c.aclCreationQueue.creations = append(c.aclCreationQueue.creations, create)
	c.aclCreationQueue.waitChans = append(c.aclCreationQueue.waitChans, make(chan error))

	var waitChan = c.aclCreationQueue.waitChans[len(c.aclCreationQueue.waitChans)-1]
	c.aclCreationQueue.timer = time.AfterFunc(c.aclCreationQueue.after, func() {
		c.aclCreationQueue.mutex.Lock()
		defer c.aclCreationQueue.mutex.Unlock()
		log.Printf("[INFO] Creating ACLs %v", c.aclCreationQueue.creations)
		defer func() {
			c.aclCreationQueue.timer = nil
			c.aclCreationQueue.creations = nil
			c.aclCreationQueue.waitChans = nil
		}()
		req := &sarama.CreateAclsRequest{
			Version:      c.getCreateAclsRequestAPIVersion(),
			AclCreations: c.aclCreationQueue.creations,
		}

		res, err := broker.CreateAcls(req)
		if err != nil {
			for _, wc := range c.aclCreationQueue.waitChans {
				wc <- err
			}
			return
		}
		if len(res.AclCreationResponses) != len(c.aclCreationQueue.waitChans) {
			for _, ch := range c.aclCreationQueue.waitChans {
				ch <- fmt.Errorf("unexpectedly got a different length (%d) of AclCreationResponses compared to queued requests (%d) - this shouldn't be possible", len(res.AclCreationResponses), len(c.aclCreationQueue.waitChans))
			}
			return
		}

		c.InvalidateACLCache()

		for i, r := range res.AclCreationResponses {
			if r.Err != sarama.ErrNoError {
				c.aclCreationQueue.waitChans[i] <- r.Err
			} else {
				c.aclCreationQueue.waitChans[i] <- nil
			}
		}

	})

	c.aclCreationQueue.mutex.Unlock()
	return <-waitChan
}

func (c *Client) CreateACL(s StringlyTypedACL) error {
	broker, err := c.client.Controller()
	if err != nil {
		return err
	}

	ac, err := tfToAclCreation(s)
	if err != nil {
		return err
	}

	err = c.enqueueCreateACL(broker, ac)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Created ACL %s", s)

	return nil
}

func stringToACLResource(in string) (out sarama.AclResourceType) {
	if err := out.UnmarshalText([]byte(in)); err == nil && out.String() == in { // Forces case-sensitive comparison
		return
	}
	return unknownConversion
}

func ACLResourceToString(in sarama.AclResourceType) string {
	if in == sarama.AclResourceUnknown {
		return "unknownConversion"
	}
	return in.String()
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

func (c *Client) InvalidateACLCache() {
	c.aclCache.mutex.Lock()
	c.aclCache.valid = false
	c.aclCache.acls = nil
	c.aclCache.mutex.Unlock()
}

func (c *Client) ListACLs() ([]*sarama.ResourceAcls, error) {
	c.aclCache.mutex.RLock()
	if c.aclCache.valid {
		c.aclCache.mutex.RUnlock()
		log.Printf("[INFO] Using cached ACL list")
		return c.aclCache.acls, nil
	}
	c.aclCache.mutex.RUnlock()

	c.aclCache.mutex.Lock()
	defer c.aclCache.mutex.Unlock()
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
	c.aclCache.valid = true
	c.aclCache.acls = res
	return res, err
}
