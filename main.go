package main

import (
	"github.com/frankfarrell/terraform-provider-redshift/redshift"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return redshift.Provider()
		},
	})
}
