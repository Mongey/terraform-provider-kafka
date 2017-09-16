package sarama

type DeleteTopicResponse struct {
	Topic string
	Err   KError
}

type DeleteTopicsResponse struct {
	DeleteTopicResponses []*DeleteTopicResponse
}

func (ct *DeleteTopicsResponse) decode(pd packetDecoder, version int16) error {
	responseCount, err := pd.getArrayLength()
	if err != nil {
		return err
	}

	ct.DeleteTopicResponses = make([]*DeleteTopicResponse, responseCount)

	for i := range ct.DeleteTopicResponses {
		ct.DeleteTopicResponses[i] = new(DeleteTopicResponse)

		topic, err := pd.getString()
		if err != nil {
			return err
		}
		ct.DeleteTopicResponses[i].Topic = topic

		tmp, err := pd.getInt16()
		if err != nil {
			return err
		}
		ct.DeleteTopicResponses[i].Err = KError(tmp)
	}

	return nil
}

func (ct *DeleteTopicsResponse) encode(pe packetEncoder) error {
	if err := pe.putArrayLength(len(ct.DeleteTopicResponses)); err != nil {
		return err
	}

	for i := range ct.DeleteTopicResponses {
		if err := pe.putString(ct.DeleteTopicResponses[i].Topic); err != nil {
			return err
		}

		pe.putInt16(int16(ct.DeleteTopicResponses[i].Err))
	}

	return nil
}
