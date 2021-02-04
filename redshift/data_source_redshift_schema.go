package redshift

import (
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRedshiftSchema() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRedshiftSchemaReadByName,

		Schema: map[string]*schema.Schema{
			"schema_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"owner": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func dataSourceRedshiftSchemaReadByName(d *schema.ResourceData, meta interface{}) error {
	var (
		oid   int
		owner int
	)

	name := d.Get("schema_name").(string)
	redshiftClient := meta.(*Client).db

	err := redshiftClient.QueryRow("select oid, nspowner from pg_namespace where nspname = $1", name).Scan(&oid, &owner)

	if err != nil {
		log.Print(err)
		return err
	}

	d.SetId(strconv.Itoa(oid))
	d.Set("owner", owner)

	return err
}
