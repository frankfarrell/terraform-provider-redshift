package redshift

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Description: "Redshift url",
				Required:    true,
			},
			"user": {
				Type:        schema.TypeString,
				Description: "master user",
				Required:    true,
			},
			"password": {
				Type:        schema.TypeString,
				Description: "master password",
				Required:    true,
				Sensitive:   true,
			},
			"port": {
				Type:        schema.TypeString,
				Description: "port",
				Optional:    true,
				Default:     "5439",
			},
			"sslmode": {
				Type:        schema.TypeString,
				Description: "SSL mode (require, disable, verify-ca, verify-full)",
				Optional:    true,
				Default:     "require",
			},
			"database": {
				Type:        schema.TypeString,
				Description: "default database",
				Optional:    true,
				Default:     "dev",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"redshift_user":                   redshiftUser(),
			"redshift_group":                  redshiftGroup(),
			"redshift_database":               redshiftDatabase(),
			"redshift_schema":                 redshiftSchema(),
			"redshift_group_schema_privilege": redshiftSchemaGroupPrivilege(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"redshift_schema": dataSourceRedshiftSchema(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	config := Config{
		url:      d.Get("url").(string),
		user:     d.Get("user").(string),
		password: d.Get("password").(string),
		port:     d.Get("port").(string),
		sslmode:  d.Get("sslmode").(string),
		database: d.Get("database").(string),
	}

	log.Println("[INFO] Initializing Redshift client")
	client, err := config.Client()
	if err != nil {
		return nil, err
	}

	db := client.db

	// DB connection is not opened until the first query.
	// Test the connection
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("Redshift connection error: %v", err)
	}

	return client, nil
}
