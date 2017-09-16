package sarama

type AlterConfigRequest struct {
	Resources    []AlterConfigResource
	ValidateOnly bool
}

type AlterConfigResource struct {
	Type          int8
	Name          string
	ConfigEntries []ConfigEntry
}

type ConfigEntry struct {
	Name  string
	Value string
}

func (acr *AlterConfigRequest) key() int16 {
	return 33
}

func (acr *AlterConfigRequest) version() int16 {
	return 0
}

func (acr *AlterConfigRequest) requiredVersion() KafkaVersion {
	return V0_11_0_0
}

func (acr *AlterConfigRequest) decode(pd packetDecoder, version int16) error {
	resourceCount, err := pd.getArrayLength()
	if err != nil {
		return err
	}
	if resourceCount == 0 {
		return nil
	}

	acr.Resources = make([]AlterConfigResource, resourceCount)
	for i := range acr.Resources {
		acr.Resources[i] = AlterConfigResource{}
		err = acr.Resources[i].decode(pd, version)
		if err != nil {
			return err
		}
	}

	validateOnly, err := pd.getBool()
	if err != nil {
		return err
	}

	acr.ValidateOnly = validateOnly

	return nil
}
func (acr *AlterConfigRequest) encode(pe packetEncoder) error {
	if err := pe.putArrayLength(len(acr.Resources)); err != nil {
		return err
	}

	for _, r := range acr.Resources {
		if err := r.encode(pe); err != nil {
			return err
		}
	}

	pe.putBool(acr.ValidateOnly)
	return nil
}

func (c *ConfigEntry) encode(pe packetEncoder) error {
	if err := pe.putString(c.Name); err != nil {
		return err
	}
	if err := pe.putString(c.Value); err != nil {
		return err
	}
	return nil
}

func (c *ConfigEntry) decode(pe packetDecoder, version int16) error {
	name, err := pe.getString()
	if err != nil {
		return err
	}
	c.Name = name

	value, err := pe.getString()
	if err != nil {
		return err
	}
	c.Value = value

	return nil
}

func (ac *AlterConfigResource) encode(pe packetEncoder) error {
	pe.putInt8(ac.Type)

	if err := pe.putString(ac.Name); err != nil {
		return err
	}

	if err := pe.putArrayLength(len(ac.ConfigEntries)); err != nil {
		return err
	}

	for _, r := range ac.ConfigEntries {
		if err := r.encode(pe); err != nil {
			return err
		}
	}
	return nil
}
func (ac *AlterConfigResource) decode(pd packetDecoder, version int16) error {
	t, err := pd.getInt8()
	if err != nil {
		return err
	}
	ac.Type = t

	name, err := pd.getString()
	if err != nil {
		return err
	}
	ac.Name = name

	configCount, err := pd.getArrayLength()
	if err != nil {
		return err
	}
	if configCount == 0 {
		return nil
	}

	ac.ConfigEntries = make([]ConfigEntry, configCount)
	for _, r := range ac.ConfigEntries {
		if err := r.decode(pd, version); err != nil {
			return err
		}
	}
	return nil
}

// maybe lies
//https://cwiki.apache.org/confluence/display/KAFKA/KIP-133%3A+Describe+and+Alter+Configs+Admin+APIs
var (
	UNKNOWN_TYPE = int8(0)
	ANY_TYPE     = int8(1)
	TOPIC_TYPE   = int8(2)
	GROUP_TYPE   = int8(3)
	CLUSTER_TYPE = int8(4)
	BROKER_TYPE  = int8(5)
)
