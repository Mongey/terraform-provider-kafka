module github.com/Mongey/terraform-provider-kafka

go 1.12

require (
	github.com/Shopify/sarama v1.22.1
	github.com/hashicorp/go-uuid v1.0.1
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/terraform v0.12.8
	github.com/hashicorp/yamux v0.0.0-20181012175058-2f1d1f20f75d // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c
)

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
