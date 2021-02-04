package redshift

//https://docs.aws.amazon.com/redshift/latest/dg/r_CREATE_DATABASE.html
//https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_DATABASE.html
//https://docs.aws.amazon.com/redshift/latest/dg/r_ALTER_DATABASE.html

import (
	"database/sql"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func redshiftDatabase() *schema.Resource {
	return &schema.Resource{
		Create: resourceRedshiftDatabaseCreate,
		Read:   resourceRedshiftDatabaseRead,
		Update: resourceRedshiftDatabaseUpdate,
		Delete: resourceRedshiftDatabaseDelete,
		Exists: resourceRedshiftDatabaseExists,
		Importer: &schema.ResourceImporter{
			State: resourceRedshiftDatabaseImport,
		},

		Schema: map[string]*schema.Schema{
			"database_name": { //This isn't immutable. The datid returned should be used as the id
				Type:     schema.TypeString,
				Required: true,
			},
			"owner": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"connection_limit": { //Cluster limit is 500
				Type:     schema.TypeString,
				Optional: true,
				Default:  "UNLIMITED",
			},
		},
	}
}

func resourceRedshiftDatabaseExists(d *schema.ResourceData, meta interface{}) (b bool, e error) {
	// Exists - This is called to verify a resource still exists. It is called prior to Read,
	// and lowers the burden of Read to be able to assume the resource exists.
	client := meta.(*Client).db

	var name string

	err := client.QueryRow("SELECT datname FROM pg_database_info WHERE datid = $1", d.Id()).Scan(&name)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

func resourceRedshiftDatabaseCreate(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db

	var createStatement string = "create database " + d.Get("database_name").(string)

	//If no owner is specified it defaults to client user
	if v, ok := d.GetOk("owner"); ok {
		var usernames = GetUsersnamesForUsesysid(redshiftClient, []interface{}{v.(int)})
		createStatement += " OWNER " + usernames[0]
	}

	if v, ok := d.GetOk("connection_limit"); ok {
		createStatement += " CONNECTION LIMIT " + v.(string)
	}

	log.Print("Create database statement: " + createStatement)

	if _, err := redshiftClient.Exec(createStatement); err != nil {
		log.Print(err)
		return err
	}

	//The changes do not propagate instantly
	time.Sleep(5 * time.Second)

	var datid string
	err := redshiftClient.QueryRow("SELECT datid FROM pg_database_info WHERE datname = $1", d.Get("database_name").(string)).Scan(&datid)

	if err != nil {
		log.Print(err)
		return err
	}

	d.SetId(datid)

	readErr := readRedshiftDatabase(d, redshiftClient)

	return readErr
}

func resourceRedshiftDatabaseRead(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db

	err := readRedshiftDatabase(d, redshiftClient)

	return err
}

func readRedshiftDatabase(d *schema.ResourceData, db *sql.DB) error {
	var (
		databasename string
		owner        int
		connlimit    sql.NullString
	)

	err := db.QueryRow("select datname, datdba, datconnlimit from pg_database_info where datid = $1", d.Id()).Scan(&databasename, &owner, &connlimit)

	if err != nil {
		log.Print(err)
		return err
	}

	d.Set("database_name", databasename)
	d.Set("owner", owner)

	if connlimit.Valid {
		d.Set("connection_limit", connlimit.String)
	} else {
		d.Set("connection_limit", nil)
	}

	return nil
}

func resourceRedshiftDatabaseUpdate(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db
	tx, txErr := redshiftClient.Begin()
	if txErr != nil {
		panic(txErr)
	}

	if d.HasChange("database_name") {

		oldName, newName := d.GetChange("database_name")
		alterDatabaseNameQuery := "ALTER DATABASE " + oldName.(string) + " rename to " + newName.(string)

		if _, err := tx.Exec(alterDatabaseNameQuery); err != nil {
			return err
		}
	}

	if d.HasChange("owner") {

		var username = GetUsersnamesForUsesysid(redshiftClient, []interface{}{d.Get("owner").(int)})

		if _, err := tx.Exec("ALTER DATABASE " + d.Get("database_name").(string) + " OWNER TO " + username[0]); err != nil {
			return err
		}
	}

	//TODO What if value is removed?
	if d.HasChange("connection_limit") {
		if _, err := tx.Exec("ALTER DATABASE " + d.Get("database_name").(string) + " CONNECTION LIMIT " + d.Get("connection_limit").(string)); err != nil {
			return err
		}
	}

	err := readRedshiftDatabase(d, redshiftClient)

	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func resourceRedshiftDatabaseDelete(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*Client).db

	_, err := client.Exec("drop database " + d.Get("database_name").(string))

	if err != nil {
		log.Print(err)
		return err
	}

	return nil
}

func resourceRedshiftDatabaseImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceRedshiftDatabaseRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}
