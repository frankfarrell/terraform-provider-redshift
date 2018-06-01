package redshift

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strings"
	"time"
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

	var name string

	err := client.QueryRow("SELECT usename FROM pg_user_info WHERE usesysid = $1", d.Id()).Scan(&name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func resourceRedshiftUserCreate(d *schema.ResourceData, meta interface{}) error {
	redshiftClient := meta.(*sql.DB)

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	var createStatement string = "create user " + d.Get("username").(string) + " with password '" + d.Get("password").(string) + "' "

	if v, ok := d.GetOk("password_disabled"); ok && v.(bool) {
		createStatement += " DISABLED "
	}
	if v, ok := d.GetOk("valid_until"); ok {
		//TODO Validate v is in format YYYY-mm-dd
		createStatement += "VALID UNTIL '" + v.(string) + "'"
	}
	if v, ok := d.GetOk("createdb"); ok {
		if v.(bool) {
			createStatement += " CREATEDB "
		} else {
			createStatement += " NOCREATEDB "
		}
	}
	if v, ok := d.GetOk("connection_limit"); ok {
		createStatement += " CONNECTION LIMIT " + v.(string)
	}
	if v, ok := d.GetOk("syslog_access"); ok {
		if v.(string) == "UNRESTRICTED" {
			createStatement += " SYSLOG ACCESS UNRESTRICTED "
		} else if v.(string) == "RESTRICTED" {
			createStatement += " SYSLOG ACCESS RESTRICTED "
		} else {
			log.Fatalf("%v is not a valid value for SYSLOG ACCESS", v)
			panic(v.(string))
		}
	}
	if v, ok := d.GetOk("superuser"); ok && v.(bool) {
		createStatement += " CREATEUSER "
	}

	if _, err := tx.Exec(createStatement); err != nil {
		log.Fatal(err)
		return err
	}

	//The changes do not propagate instantly
	time.Sleep(5 * time.Second)

	var usesysid string
	err := tx.QueryRow("SELECT usesysid FROM pg_user_info WHERE usename = $1", d.Get("username").(string)).Scan(&usesysid)

	if err != nil {
		log.Fatal(err)
		return err
	}

	d.SetId(usesysid)

	readErr := readRedshiftUser(d, tx)

	if readErr == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return readErr
	}

}

func resourceRedshiftUserRead(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*sql.DB)

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	err := readRedshiftUser(d, tx)

	if err == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return err
	}
}

func readRedshiftUser(d *schema.ResourceData, tx *sql.Tx) error {

	var (
		usename      string
		usecreatedb  bool
		usesuper     bool
		valuntil     sql.NullString
		useconnlimit sql.NullString
	)

	err := tx.QueryRow("select usename, usecreatedb, usesuper, valuntil, useconnlimit "+
		"from pg_user_info where usesysid = $1", d.Id()).Scan(&usename, &usecreatedb, &usesuper, &valuntil, &useconnlimit)

	if err != nil {
		log.Fatal(err)
		return err
	}

	d.Set("username", usename)
	d.Set("createdb", usecreatedb)
	d.Set("superuser", usesuper)

	if valuntil.Valid {
		d.Set("valid_until", valuntil.String)
	} else {
		d.Set("valid_until", nil)
	}

	if useconnlimit.Valid {
		d.Set("connection_limit", useconnlimit.String)
	} else {
		d.Set("connection_limit", nil)
	}

	return nil
}

func resourceRedshiftUserUpdate(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*sql.DB)
	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	if d.HasChange("username") {

		oldUsername, newUsername := d.GetChange("username")
		alterUserQuery := "alter user " + oldUsername.(string) + " rename to " + newUsername.(string)

		if _, err := tx.Exec(alterUserQuery); err != nil {
			return err
		}

		//If name changes we also need to reset the password
		if err := resetPassword(tx, d, newUsername.(string)); err != nil {
			return err
		}
	} else if d.HasChange("password") || d.HasChange("password_disabled") || d.HasChange("valid_until") {
		if err := resetPassword(tx, d, d.Get("username").(string)); err != nil {
			return err
		}
	}

	if d.HasChange("createdb") {

		if v, ok := d.GetOk("createdb"); ok && v.(bool) {
			if _, err := tx.Exec("alter user " + d.Get("username").(string) + " createdb"); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("alter user " + d.Get("username").(string) + " nocreatedb"); err != nil {
				return err
			}
		}
	}
	//TODO What if value is removed?
	if d.HasChange("connection_limit") {
		if _, err := tx.Exec("alter user " + d.Get("username").(string) + " CONNECTION LIMIT " + d.Get("connection_limit").(string)); err != nil {
			return err
		}
	}
	if d.HasChange("syslog_access") {
		if _, err := tx.Exec("alter user " + d.Get("username").(string) + " SYSLOG ACCESS " + d.Get("syslog_access").(string)); err != nil {
			return err
		}
	}
	if d.HasChange("superuser") {
		if v, ok := d.GetOk("superuser"); ok && v.(bool) {
			if _, err := tx.Exec("alter user " + d.Get("username").(string) + " CREATEUSER "); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("alter user " + d.Get("username").(string) + " NOCREATEUSER"); err != nil {
				return err
			}
		}
	}

	err := readRedshiftUser(d, tx)

	if err == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return err
	}
}

func resetPassword(tx *sql.Tx, d *schema.ResourceData, username string) error {

	if v, ok := d.GetOk("password_disabled"); ok && v.(bool) {

		var disablePasswordQuery = "alter user " + username + " password disable"

		if _, err := tx.Exec(disablePasswordQuery); err != nil {
			return err
		}
		return nil

	} else {
		var resetPasswordQuery = "alter user " + username + " password '" + d.Get("password").(string) + "' "
		if v, ok := d.GetOk("valid_until"); ok {
			resetPasswordQuery += " VALID UNTIL '" + v.(string) + "'"

		}
		if _, err := tx.Exec(resetPasswordQuery); err != nil {
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

	_, err := client.Exec("drop user " + d.Get("username").(string))

	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func resourceRedshiftUserImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceRedshiftUserRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

func GetUsersnamesForUsesysid(tx *sql.Tx, usersIdsInterface []interface{}) []string {

	var usersIds = make([]int, 0)

	for _, v := range usersIdsInterface {
		usersIds = append(usersIds, v.(int))
	}

	var usernames []string

	//I couldnt figure out how to pass a slice to go sql
	var selectUserQuery string = fmt.Sprintf("select usename from pg_user_info where usesysid in (%s)", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(usersIds)), ","), "[]"))

	log.Print("Select user query: " + selectUserQuery)

	rows, err := tx.Query(selectUserQuery)
	if err != nil {
		// handle this error better than this
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var username string
		err = rows.Scan(&username)
		if err != nil {
			// handle this error
			panic(err)
		}

		usernames = append(usernames, username)
	}
	// get any error encountered during iteration
	err = rows.Err()
	if err != nil {
		panic(err)
	}

	return usernames
}
