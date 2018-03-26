package terraform_provider_redshift

import (
	"github.com/frankfarrell/terraform-provider-redshift/redshift"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: redshift.Provider})
}
