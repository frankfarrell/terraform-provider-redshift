package terraform_provider_redshift

import (
	"github.com/hashicorp/terraform/plugin"
	"github.com/frankfarrell/terraform-provider-redshift/redshift"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: redshift.Provider})
}