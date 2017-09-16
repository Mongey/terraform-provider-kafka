package main

import (
	"github.com/Mongey/terraform-kafka/kafka"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: kafka.Provider})
}
