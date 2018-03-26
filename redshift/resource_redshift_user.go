package redshift

import (
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"database/sql"
)

func redshiftUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceRedshiftUserCreate,
		Read:   resourceRedshiftUserRead,
		Update: resourceRedshiftUserUpdate,
		Delete: resourceRedshiftUserDelete,
		Exists: resourceRedshiftUserExists,
		Importer: &schema.ResourceImporter{
			State: resourceRedshiftUserImport,
		},

		Schema: map[string]*schema.Schema{
			"username": { //This isn't immutable. The usesysid returned should be used as the id
				Type:     schema.TypeString,
				Required: true,
			},
			"password": { //Can we read this back from the db? If not hwo can we tell if its changed? Do we need to use md5hash?
				Type:     schema.TypeString,
				Required: true,
			},
			"valid_until": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"password_disabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"createdb": { //Allows them to create databases
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"connection_limit": { //Cluster limit is 500 anyway
				Type:     schema.TypeString,
				Optional: true,
				Default:  "UNLIMITED",
			},
			"syslog_access": { //Can be RESTRICTED | UNRESTRICTED
				Type:     schema.TypeString,
				Optional: true,
				Default:  "RESTRICTED",
			},
			"superuser": { //If true set CREATEUSER
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"usesysid": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceRedshiftUserExists(d *schema.ResourceData, meta interface{}) (b bool, e error) {
	// Exists - This is called to verify a resource still exists. It is called prior to Read,
	// and lowers the burden of Read to be able to assume the resource exists.
	client := meta.(*sql.DB)

	err := client.QueryRow("SELECT * FROM pg_user_info WHERE usesysid = ?", d.Id()).Scan()
	if err != nil {
		return false, err
	}
	return true, nil
}

func resourceRedshiftUserCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*sql.DB)

	var createStatement = "create user " + d.Get("username") +" with password ? "

	if v, ok := d.GetOk("password_disabled"); v && ok{
		createStatement += " DISABLED "
	}
	if v, ok := d.GetOk("valid_until"); ok{
		//TODO Validate v is in format YYYY-mm-dd
		createStatement += "VALID UN TIL " + v
	}
	if v, ok := d.GetOk("createdb"); ok{
		if v {
			createStatement += " CREATEDB "
		} else {
			createStatement += " NOCREATEDB "
		}
	}
	if v, ok := d.GetOk("connection_limit"); ok{
		createStatement += " CONNECTION LIMIT " + v
	}
	if v, ok := d.GetOk("syslog_access"); ok{
		if v == "UNRESTRICTED" {
			createStatement += " SYSLOG ACCESS UNRESTRICTED "
		} else if v == "RESTRICTED"{
			createStatement += " SYSLOG ACCESS RESTRICTED "
		} else {
			log.Fatalf("%v is not a valid value for SYSLOG ACCESS", v)
			panic(v)
		}
	}
	if v, ok := d.GetOk("superuser"); v && ok{
		createStatement += " CREATEUSER "
	}

	_, err := client.Exec(createStatement, d.Get("username"), d.Get("password"))

	if _, err := client.Exec(createStatement, d.Get("password")); err != nil {
		log.Fatal(err)
		return false, err
	}

	var usesysid string
	err = client.QueryRow("SELECT usesysid FROM pg_user_info WHERE usename = ?", d.Get("username")).Scan(&usesysid)

	if err != nil {
		log.Fatal(err)
		return false, err
	}

	d.SetId(usesysid)

	return resourceRedshiftUserRead(d, meta)
}

func resourceRedshiftUserRead(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*sql.DB)

	var (
		usename string
		usecreatedb bool
		usesuper bool
		valuntil string
		useconnlimit string
	)

	err := client.QueryRow("select usename, usecreatedb, usesuper, valuntil, useconnlimit " +
		"from pg_user_info where usesysid = ?", d.Id()).Scan(&usename, &usecreatedb, &usesuper, &valuntil, &useconnlimit)

	if err != nil {
		log.Fatal(err)
		return false, err
	}

	//TODO Figure these out her
	d.Set("username", usename)
	if valuntil != nil {
		d.Set("valid_until", valuntil)
	}
	if useconnlimit != nil {
		d.Set("valid_until", valuntil)
	}
	d.Set("createdb", usecreatedb)
	d.Set("connection_limit", useconnlimit)

	return nil
}

func resourceRedshiftUserUpdate(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*sql.DB)

	if d.HasChange("username") {

		oldUsername, newUsername := d.GetChange("username")
		alterUserQuery := "alter user "+oldUsername + "rename to " + newUsername

		if _, err := client.Exec(alterUserQuery); err {
			return err
		}

		//If name changes we also need to reset the password
		if err := resetPassword(client, d, newUsername); err {
			return err
		}
	} else if d.HasChange("password") || d.HasChange("password_disabled") || d.HasChange("valid_until") {
		if err := resetPassword(client, d, d.Get("username")); err {
			return err
		}
	}

	if d.HasChange("createdb") {

		if v, ok := d.GetOk("createdb"); v && ok{
			if _, err := client.Exec("alter user " + d.Get("username") + " createdb"); err {
				return err
			}
		} else {
			if _, err := client.Exec("alter user " + d.Get("username") + " nocreatedb"); err {
				return err
			}
		}
	}
	//TODO What if value is removed?
	if d.HasChange("connection_limit") {
		if _, err := client.Exec("alter user " + d.Get("username") + " CONNECTION LIMIT " + d.Get("connection_limit")); err {
			return err
		}
	}
	if d.HasChange("syslog_access") {
		if _, err := client.Exec("alter user " + d.Get("username") + " SYSLOG ACCESS " + d.Get("syslog_access")); err {
			return err
		}
	}
	if d.HasChange("superuser") {
		if v, ok := d.GetOk("superuser"); v && ok{
			if _, err := client.Exec("alter user " + d.Get("username") + " CREATEUSER "); err {
				return err
			}
		} else {
			if _, err := client.Exec("alter user " + d.Get("username") + " NOCREATEUSER"); err {
				return err
			}
		}
	}

	return resourceRedshiftUserRead(d, meta)
}

func resetPassword (client *sql.DB, d *schema.ResourceData, username string) error {

	if v, ok := d.GetOk("password_disabled"); v && ok {

		var disablePasswordQuery = "alter user " + username +" password disable"

		if _, err :=client.Exec(disablePasswordQuery); err {
			return err
		}
		return nil

	} else {
		var resetPasswordQuery = "alter user " + username +" password ?"
		if v, ok := d.GetOk("valid_until");ok {
			resetPasswordQuery += " VALID UNTIL " + v

		}
		if _, err :=client.Exec(resetPasswordQuery, d.Get("password")); err {
			return err
		}
		return nil
	}
}

func resourceRedshiftUserDelete(d *schema.ResourceData, meta interface{}) error {

	/*
	NB BIG TODO!!!

	https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_USER.html
	If a user owns an object, first drop the object or change its ownership to another user before dropping
	the original user. If the user has privileges for an object, first revoke the privileges before dropping
	the user. The following example shows dropping an object, changing ownership, and revoking privileges
	before dropping the user

	Ideally, these dependencies would be managed by TF. So a schema has an owner (which can be changed without force destroy)
	But you can't have a schema without an owner for instance

	So suppose you delete bob who owns schema X.
	First change owner of schema X to mary.
	Then delete Bob
	Permissions will be modelled as TF resources too.
	 */

	client := meta.(*sql.DB)

	_, err := client.Exec("drop user " + d.Get("username"))

	if err != nil {
		log.Fatal(err)
		return false, err
	}

	return nil
}

func resourceRedshiftUserImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceRedshiftUserRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}