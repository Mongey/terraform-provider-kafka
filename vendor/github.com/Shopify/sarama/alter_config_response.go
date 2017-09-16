package sarama

type AlterConfigResponse struct {
	TrottleMs int32
	Resources []*AlterConfigResourceResponse
}

type AlterConfigResourceResponse struct {
	ErrorCode    int16
	ErrorMessage string
	Type         int8
	Name         string
}

func (acr *AlterConfigResponse) decode(pd packetDecoder, version int16) error {
	trottle, err := pd.getInt32()
	if err != nil {
		return err
	}
	acr.TrottleMs = trottle

	responseCount, err := pd.getArrayLength()
	if err != nil {
		return err
	}

	acr.Resources = make([]*AlterConfigResourceResponse, responseCount)

	for i := range acr.Resources {
		acr.Resources[i] = new(AlterConfigResourceResponse)

		errCode, err := pd.getInt16()
		if err != nil {
			return err
		}
		acr.Resources[i].ErrorCode = errCode

		e, err := pd.getString()
		if err != nil {
			return err
		}
		acr.Resources[i].ErrorMessage = e

		t, err := pd.getInt8()
		if err != nil {
			return err
		}
		acr.Resources[i].Type = t

		name, err := pd.getString()
		if err != nil {
			return err
		}
		acr.Resources[i].Name = name
	}

	return nil
}

func (ct *AlterConfigResponse) encode(pe packetEncoder) error {
	pe.putInt32(ct.TrottleMs)

	if err := pe.putArrayLength(len(ct.Resources)); err != nil {
		return err
	}

	for i := range ct.Resources {
		pe.putInt16(ct.Resources[i].ErrorCode)
		err := pe.putString(ct.Resources[i].ErrorMessage)
		if err != nil {
			return nil
		}
		pe.putInt8(ct.Resources[i].Type)
		err = pe.putString(ct.Resources[i].Name)
		if err != nil {
			return nil
		}
	}

	return nil
}
