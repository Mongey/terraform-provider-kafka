module github.com/Mongey/terraform-provider-kafka

go 1.16

require (
	github.com/Shopify/sarama v1.35.0
	github.com/hashicorp/go-cty v1.4.1-0.20200414143053-d3edf31b6320
	github.com/hashicorp/go-uuid v1.0.3
	github.com/hashicorp/terraform-plugin-docs v0.10.1
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.17.0
	github.com/xdg/scram v1.0.5
	github.com/xdg/stringprep v1.0.3 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d // indirect
)

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
