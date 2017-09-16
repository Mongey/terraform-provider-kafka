package sarama

type DeleteTopicsRequest struct {
	DeleteTopicRequests []DTopic
	Timeout             int32
}

type DTopic struct {
	Topic string
}

func (dr *DTopic) encode(pe packetEncoder) error {
	return pe.putString(dr.Topic)
}

func (dr *DTopic) decode(pd packetDecoder, version int16) error {
	topic, err := pd.getString()
	if err != nil {
		return err
	}

	dr.Topic = topic

	return nil
}

func (dt *DeleteTopicsRequest) encode(pe packetEncoder) error {
	if err := pe.putArrayLength(len(dt.DeleteTopicRequests)); err != nil {
		return err
	}

	for i := range dt.DeleteTopicRequests {
		if err := dt.DeleteTopicRequests[i].encode(pe); err != nil {
			return err
		}
	}

	pe.putInt32(dt.Timeout)

	return nil
}

func (dt *DeleteTopicsRequest) decode(pd packetDecoder, version int16) error {
	deleteRequestCount, err := pd.getArrayLength()
	if err != nil {
		return err
	}
	if deleteRequestCount == 0 {
		return nil
	}

	dt.DeleteTopicRequests = make([]DTopic, deleteRequestCount)
	for i := range dt.DeleteTopicRequests {
		dt.DeleteTopicRequests[i] = DTopic{}
		err = dt.DeleteTopicRequests[i].decode(pd, version)
		if err != nil {
			return err
		}
	}

	timeout, err := pd.getInt32()
	if err != nil {
		return err
	}

	dt.Timeout = timeout

	return nil
}

func (ct *DeleteTopicsRequest) key() int16 {
	return 20
}

func (ct *DeleteTopicsRequest) version() int16 {
	return 0
}

func (ct *DeleteTopicsRequest) requiredVersion() KafkaVersion {
	return V0_10_0_0
}
