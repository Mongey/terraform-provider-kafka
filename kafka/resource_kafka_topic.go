package kafka

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func positiveValue(val interface{}, key string) (warns []string, errs []error) {
	v := val.(int)
	if v < 1 {
		errs = append(errs, fmt.Errorf("%q must be greater than 0, got: %d", key, v))
	}
	return
}

func kafkaTopicResource() *schema.Resource {
	return &schema.Resource{
		Create: topicCreate,
		Read:   topicRead,
		Update: topicUpdate,
		Delete: topicDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		CustomizeDiff: customDiff,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the topic.",
			},
			"partitions": {
				Type:         schema.TypeInt,
				Required:     true,
				Description:  "Number of partitions.",
				ValidateFunc: positiveValue,
			},
			"replication_factor": {
				Type:         schema.TypeInt,
				Required:     true,
				ForceNew:     false,
				Description:  "Number of replicas.",
				ValidateFunc: positiveValue,
			},
			"config": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    false,
				Description: "A map of string k/v attributes.",
				Elem:        schema.TypeString,
			},
		},
	}
}

func topicCreate(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*LazyClient)
	t := metaToTopic(d, meta)

	err := c.CreateTopic(t)
	if err != nil {
		return err
	}

	stateConf := &resource.StateChangeConf{
		Pending: []string{"Pending"},
		Target: []string{"Created"},
		Refresh: topicCreateFunc(c, t),
		Timeout: time.Duration(c.Config.Timeout) * time.Second,
		Delay: 1 * time.Second,
		PollInterval: 2 * time.Second,
	}

	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf("error waiting for topic (%s) to be created: %s", t.Name, err)
	}

	d.SetId(t.Name)
	return nil
}

func topicCreateFunc(client *LazyClient, t Topic) resource.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		topic, err := client.ReadTopic(t.Name, true)
		switch e := err.(type) {
		case TopicMissingError:
			return topic, "Pending", nil
		case nil:
			return topic, "Created", nil
		default:
			return topic, "Error", e
		}
	}
}

func topicUpdate(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*LazyClient)
	t := metaToTopic(d, meta)

	if err := c.UpdateTopic(t); err != nil {
		return err
	}

	// update replica count of existing partitions before adding new ones
	if d.HasChange("replication_factor") {
		oi, ni := d.GetChange("replication_factor")
		oldRF := oi.(int)
		newRF := ni.(int)
		log.Printf("[INFO] Updating replication_factor from %d to %d", oldRF, newRF)
		t.ReplicationFactor = int16(newRF)

		if err := c.AlterReplicationFactor(t); err != nil {
			return err
		}

		if err := waitForRFUpdate(c, d.Id()); err != nil {
			return err
		}
	}

	if d.HasChange("partitions") {
		// update should only be called when we're increasing partitions
		oi, ni := d.GetChange("partitions")
		oldPartitions := oi.(int)
		newPartitions := ni.(int)
		log.Printf("[INFO] Updating partitions from %d to %d", oldPartitions, newPartitions)
		t.Partitions = int32(newPartitions)

		if err := c.AddPartitions(t); err != nil {
			return err
		}
	}

	if err := waitForTopicRefresh(c, d.Id(), t); err != nil {
		return err
	}

	return nil
}

func waitForRFUpdate(client *LazyClient, topic string) error {
	refresh := func() (interface{}, string, error) {
		isRFUpdating, err := client.IsReplicationFactorUpdating(topic)
		if err != nil {
			return nil, "Error", err
		} else if isRFUpdating {
			return nil, "Updating", nil
		} else {
			return "not-nil", "Ready", nil
		}
	}

	timeout := time.Duration(client.Config.Timeout) * time.Second
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"Updating"},
		Target:       []string{"Ready"},
		Refresh:      refresh,
		Timeout:      timeout,
		Delay:        1 * time.Second,
		PollInterval: 1 * time.Second,
		MinTimeout:   2 * time.Second,
	}

	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf(
			"Error waiting for topic (%s) replication_factor to update: %s",
			topic, err)
	}

	return nil
}

func waitForTopicRefresh(client *LazyClient, topic string, expected Topic) error {
	timeout := time.Duration(client.Config.Timeout) * time.Second
	stateConf := &resource.StateChangeConf{
		Pending:      []string{"Updating"},
		Target:       []string{"Ready"},
		Refresh:      topicRefreshFunc(client, topic, expected),
		Timeout:      timeout,
		Delay:        1 * time.Second,
		PollInterval: 1 * time.Second,
		MinTimeout:   2 * time.Second,
	}

	if _, err := stateConf.WaitForState(); err != nil {
		return fmt.Errorf(
			"Error waiting for topic (%s) to become ready: %s",
			topic, err)
	}

	return nil
}

func topicRefreshFunc(client *LazyClient, topic string, expected Topic) resource.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		log.Printf("[DEBUG] waiting for topic to update %s", topic)
		actual, err := client.ReadTopic(topic, true)
		if err != nil {
			log.Printf("[ERROR] could not read topic %s, %s", topic, err)
			return actual, "Error", err
		}

		if expected.Equal(actual) {
			return actual, "Ready", nil
		}

		return nil, fmt.Sprintf("%v != %v", strPtrMapToStrMap(actual.Config), strPtrMapToStrMap(expected.Config)), nil
	}
}

func topicDelete(d *schema.ResourceData, meta interface{}) error {
	c := meta.(*LazyClient)
	t := metaToTopic(d, meta)

	err := c.DeleteTopic(t.Name)
	if err != nil {
		return err
	}

	stateConf := &resource.StateChangeConf{
		Pending:      []string{"Pending"},
		Target:       []string{"Deleted"},
		Refresh:      topicDeleteFunc(c, d.Id(), t),
		Timeout:      300 * time.Second,
		Delay:        3 * time.Second,
		PollInterval: 2 * time.Second,
		MinTimeout:   20 * time.Second,
	}
	_, err = stateConf.WaitForState()

	if err != nil {
		return fmt.Errorf("Error waiting for topic (%s) to delete: %s", d.Id(), err)
	}

	d.SetId("")
	return err
}

func topicDeleteFunc(client *LazyClient, id string, t Topic) resource.StateRefreshFunc {
	return func() (result interface{}, s string, err error) {
		topic, err := client.ReadTopic(t.Name, true)

		if err != nil {
			_, ok := err.(TopicMissingError)
			if ok {
				return topic, "Deleted", nil
			}
			return topic, "UNKNOWN", err
		}
		return topic, "Pending", nil
	}

}

func topicRead(d *schema.ResourceData, meta interface{}) error {
	name := d.Id()
	client := meta.(*LazyClient)
	topic, err := client.ReadTopic(name, false)

	if err != nil {
		log.Printf("[ERROR] Error getting topics %s from Kafka", err)
		_, ok := err.(TopicMissingError)
		if ok {
			d.SetId("")
			return nil
		}

		return err
	}

	log.Printf("[DEBUG] Setting the state from Kafka %v", topic)
	errSet := errSetter{d: d}
	errSet.Set("name", topic.Name)
	errSet.Set("partitions", topic.Partitions)
	errSet.Set("replication_factor", topic.ReplicationFactor)
	errSet.Set("config", topic.Config)

	return errSet.err
}

func customDiff(diff *schema.ResourceDiff, v interface{}) error {
	log.Printf("[INFO] Checking the diff!")
	client := v.(*LazyClient)

	if diff.HasChange("partitions") {
		log.Printf("[INFO] Partitions have changed!")
		o, n := diff.GetChange("partitions")
		oi := o.(int)
		ni := n.(int)
		log.Printf("Partitions is changing from %d to %d", oi, ni)
		if ni < oi {
			log.Printf("Partitions decreased from %d to %d. Forcing new resource", oi, ni)
			if err := diff.ForceNew("partitions"); err != nil {
				return err
			}
		}
	}

	if diff.HasChange("replication_factor") {
		canAlterRF, err := client.CanAlterReplicationFactor()
		if err != nil {
			return err
		}

		if !canAlterRF {
			log.Println("[INFO] Need kafka >= 2.4.0 to update replication_factor in-place")
			if err := diff.ForceNew("replication_factor"); err != nil {
				return err
			}
		}
	}

	return nil
}
