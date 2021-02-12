package redshift

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
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
	client := meta.(*Client).db

	var name string

	err := client.QueryRow("SELECT usename FROM pg_user_info WHERE usesysid = $1", d.Id()).Scan(&name)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

func resourceRedshiftUserCreate(d *schema.ResourceData, meta interface{}) error {
	redshiftClient := meta.(*Client).db

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	var createStatement string = "create user " + d.Get("username").(string) + " with password "

	if v, ok := d.GetOk("password_disabled"); ok && v.(bool) {
		createStatement += " DISABLE "
	} else if v, ok := d.GetOk("password"); ok {
		createStatement += "'" + v.(string) + "' "
	} else {
		return fmt.Errorf("Either password_disabled attribute has to be set to true or password attribute has to be provided")
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
			log.Printf("%v is not a valid value for SYSLOG ACCESS", v)
			panic(v.(string))
		}
	}
	if v, ok := d.GetOk("superuser"); ok && v.(bool) {
		createStatement += " CREATEUSER "
	}

	if _, err := tx.Exec(createStatement); err != nil {
		return fmt.Errorf("Could not create redshift user: %s", err)
	}

	log.Print("User created, waiting 5 seconds for propagation")

	//The changes do not propagate instantly
	time.Sleep(5 * time.Second)

	var usesysid string
	err := tx.QueryRow("SELECT usesysid FROM pg_user_info WHERE usename = $1", d.Get("username").(string)).Scan(&usesysid)

	if err != nil {
		log.Print("User does not exist in pg_user_info table")
		log.Print(err)
		return err
	}

	log.Printf("usesysid for user is %s", usesysid)

	d.SetId(usesysid)

	readErr := readRedshiftUser(d, tx)

	if readErr != nil {
		tx.Rollback()
		return readErr
	}

	tx.Commit()
	return nil
}

func resourceRedshiftUserRead(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	err := readRedshiftUser(d, tx)

	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func readRedshiftUser(d *schema.ResourceData, tx *sql.Tx) error {

	var (
		usename      string
		usecreatedb  bool
		usesuper     bool
		valuntil     sql.NullString
		useconnlimit sql.NullString
	)

	var readUserQuery = "select usename, usecreatedb, usesuper, valuntil, useconnlimit " +
		"from pg_user_info where usesysid = $1"

	log.Print("Reading redshift user with query: " + readUserQuery)

	err := tx.QueryRow(readUserQuery, d.Id()).Scan(&usename, &usecreatedb, &usesuper, &valuntil, &useconnlimit)

	if err != nil {
		log.Print("Reading user does not exist")
		log.Print(err)
		return err
	}

	log.Print("Succesfully read redshift user")

	d.Set("username", usename)
	d.Set("createdb", usecreatedb)
	d.Set("superuser", usesuper)

	if valuntil.Valid {
		log.Print("Valid until " + valuntil.String)
		d.Set("valid_until", valuntil.String)
	} else {
		d.Set("valid_until", nil)
	}

	if useconnlimit.Valid {
		log.Print("User connection limit " + useconnlimit.String)
		d.Set("connection_limit", useconnlimit.String)
	} else {
		d.Set("connection_limit", nil)
	}

	return nil
}

func resourceRedshiftUserUpdate(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db
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

	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
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

	redshiftClient := meta.(*Client).db
	redshiftClientConfig := meta.(*Client).config

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		return fmt.Errorf("Could not begin redshift transaction: %s", txErr)
	}

	// https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_USER.html
	// If a user owns an object, first drop the object or change its ownership to another user before dropping
	// the original user. If the user has privileges for an object, first revoke the privileges before dropping
	// the user. The following example shows dropping an object, changing ownership, and revoking privileges
	// before dropping the user
	//
	// There is no equivalent of Postgres REASSIGN USER unfortunately
	//
	// It is necessary to query and reassign to current user.
	// See some discussion her: https://dba.stackexchange.com/questions/143938/drop-user-in-redshift-which-has-privilege-on-some-object
	//
	// Derived from https://github.com/awslabs/amazon-redshift-utils/blob/master/src/AdminViews/v_find_dropuser_objs.sql
	var reassignOwnerGenerator = `SELECT owner.ddl
		FROM (
				-- Functions owned by the user
				SELECT pgu.usesysid,
				'alter function ' || QUOTE_IDENT(nc.nspname) || '.' ||textin (regprocedureout (pproc.oid::regprocedure)) || ' owner to '
				FROM pg_proc pproc,pg_user pgu,pg_namespace nc
				WHERE pproc.pronamespace = nc.oid
				AND   pproc.proowner = pgu.usesysid
			UNION ALL
				-- Databases owned by the user
				SELECT pgu.usesysid,
				'alter database ' || QUOTE_IDENT(pgd.datname) || ' owner to '
				FROM pg_database pgd,
				     pg_user pgu
				WHERE pgd.datdba = pgu.usesysid
			UNION ALL
				-- Schemas owned by the user
				SELECT pgu.usesysid,
				'alter schema '|| QUOTE_IDENT(pgn.nspname) ||' owner to '
				FROM pg_namespace pgn,
				     pg_user pgu
				WHERE pgn.nspowner = pgu.usesysid
			UNION ALL
				-- Tables or Views owned by the user
				SELECT pgu.usesysid,
				'alter table ' || QUOTE_IDENT(nc.nspname) || '.' || QUOTE_IDENT(pgc.relname) || ' owner to '
				FROM pg_class pgc,
				     pg_user pgu,
				     pg_namespace nc
				WHERE pgc.relnamespace = nc.oid
				AND   pgc.relkind IN ('r','v')
				AND   pgu.usesysid = pgc.relowner
				AND   nc.nspname NOT ILIKE 'pg\_temp\_%'
		)
		OWNER("userid", "ddl")
		WHERE owner.userid = $1;`

	rows, reassignOwnerStatementErr := tx.Query(reassignOwnerGenerator, d.Id())

	defer rows.Close()

	if reassignOwnerStatementErr != nil {
		tx.Rollback()
		return reassignOwnerStatementErr
	}

	var reassignStatements []string

	for rows.Next() {

		var reassignStatement string

		err := rows.Scan(&reassignStatement)
		if err != nil {
			//Im not sure how this can happen
			tx.Rollback()
			return err
		}
		reassignStatements = append(reassignStatements, reassignStatement)
	}

	for _, statement := range reassignStatements {
		_, err := tx.Exec(statement + redshiftClientConfig.user)

		if err != nil {
			//Im not sure how this can happen
			tx.Rollback()
			return err
		}
	}

	//We need to drop all privileges and default privileges
	rows, schemasError := redshiftClient.Query("select nspname from pg_namespace")
	defer rows.Close()
	if schemasError != nil {
		return schemasError
	}

	for rows.Next() {
		var schemaName string
		err := rows.Scan(&schemaName)
		if err != nil {
			//Im not sure how this can happen
			return err
		}
		redshiftClient.Exec("REVOKE ALL ON ALL TABLES IN SCHEMA " + schemaName + " FROM " + d.Get("username").(string))
		redshiftClient.Exec("ALTER DEFAULT PRIVILEGES IN SCHEMA " + schemaName + " REVOKE ALL ON TABLES FROM " + d.Get("username").(string) + " CASCADE")
	}

	_, dropUserErr := tx.Exec("DROP USER " + d.Get("username").(string))

	if dropUserErr != nil {
		return dropUserErr
		tx.Rollback()
	}

	tx.Commit()
	return nil
}

func resourceRedshiftUserImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceRedshiftUserRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

type Queryer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func GetUsersnamesForUsesysid(q Queryer, usersIdsInterface []interface{}) []string {

	var usersIds = make([]int, 0)

	for _, v := range usersIdsInterface {
		usersIds = append(usersIds, v.(int))
	}

	var usernames []string

	//I couldnt figure out how to pass a slice to go sql
	var selectUserQuery = fmt.Sprintf("select usename from pg_user_info where usesysid in (%s)", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(usersIds)), ","), "[]"))

	log.Print("Select user query: " + selectUserQuery)

	rows, err := q.Query(selectUserQuery)

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
