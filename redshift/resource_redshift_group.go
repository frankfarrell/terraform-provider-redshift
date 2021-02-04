package redshift

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// https://docs.aws.amazon.com/redshift/latest/dg/r_CREATE_GROUP.html
// https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_GROUP.html
// https://docs.aws.amazon.com/redshift/latest/dg/r_ALTER_GROUP.html

func redshiftGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceRedshiftGroupCreate,
		Read:   resourceRedshiftGroupRead,
		Update: resourceRedshiftGroupUpdate,
		Delete: resourceRedshiftGroupDelete,
		Exists: resourceRedshiftGroupExists,
		Importer: &schema.ResourceImporter{
			State: resourceRedshiftGroupImport,
		},

		Schema: map[string]*schema.Schema{
			"group_name": { //This isn't immutable. The usesysid returned should be used as the id
				Type:     schema.TypeString,
				Required: true,
			},
			//Pass usesysid as username can change
			"users": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
		},
	}
}

func resourceRedshiftGroupExists(d *schema.ResourceData, meta interface{}) (b bool, e error) {
	// Exists - This is called to verify a resource still exists. It is called prior to Read,
	// and lowers the burden of Read to be able to assume the resource exists.
	client := meta.(*Client).db

	var name string

	err := client.QueryRow("SELECT groname FROM pg_group WHERE grosysid = $1", d.Id()).Scan(&name)
	switch {
	case err == sql.ErrNoRows:
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

func resourceRedshiftGroupCreate(d *schema.ResourceData, meta interface{}) error {
	redshiftClient := meta.(*Client).db

	tx, txErr := redshiftClient.Begin()
	if txErr != nil {
		panic(txErr)
	}

	var createStatement string = "create group " + d.Get("group_name").(string)
	if v, ok := d.GetOk("users"); ok {
		var usernames = GetUsersnamesForUsesysid(tx, v.(*schema.Set).List())
		createStatement += " WITH USER " + strings.Join(usernames, ", ")
	}

	log.Print("Create group statement: " + createStatement)

	if _, err := tx.Exec(createStatement); err != nil {
		return fmt.Errorf("Could not create redshift group: %s", err)
	}

	log.Print("Group created succesfully, reading grosyid from pg_group")

	//The changes do not propagate instantly
	time.Sleep(5 * time.Second)

	var grosysid int
	err := tx.QueryRow("SELECT grosysid FROM pg_group WHERE groname = $1", d.Get("group_name").(string)).Scan(&grosysid)
	if err != nil {
		return fmt.Errorf("Could not get redshift group id: %s", err)
	}

	log.Printf("grosysid is %s", strconv.Itoa(grosysid))

	d.SetId(strconv.Itoa(grosysid))

	readErr := readRedshiftGroup(d, tx)

	if readErr != nil {
		tx.Rollback()
		return readErr
	}

	tx.Commit()
	return nil
}

func resourceRedshiftGroupRead(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	err := readRedshiftGroup(d, tx)

	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func readRedshiftGroup(d *schema.ResourceData, tx *sql.Tx) error {
	var (
		groupname string
		users     sql.NullString
	)

	err := tx.QueryRow("SELECT groname, grolist FROM pg_group WHERE grosysid = $1", d.Id()).Scan(&groupname, &users)

	if err != nil {
		log.Print(err)
		return err
	}

	d.Set("group_name", groupname)

	//Notes on postgres array types https://gist.github.com/adharris/4163702, eg startying with underscore _int4

	if users.Valid {
		var userIdsAsString = strings.Split(users.String[1:len(users.String)-1], ",")
		var userIdsAsInt = []int{}

		for _, i := range userIdsAsString {
			j, err := strconv.Atoi(i)
			if err != nil {
				panic(err)
			}
			userIdsAsInt = append(userIdsAsInt, j)
		}

		d.Set("users", userIdsAsInt)
	} else {
		d.Set("users", []int{})
	}

	return nil
}

func resourceRedshiftGroupUpdate(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*Client).db
	tx, txErr := redshiftClient.Begin()
	if txErr != nil {
		panic(txErr)
	}

	if d.HasChange("group_name") {

		oldName, newName := d.GetChange("group_name")
		alterDatabaseNameQuery := "ALTER GROUP " + oldName.(string) + " RENAME TO " + newName.(string)

		if _, err := tx.Exec(alterDatabaseNameQuery); err != nil {
			return err
		}
	}

	if d.HasChange("users") {

		oldUserSet, newUserSet := d.GetChange("users")

		var usersRemoved = difference(oldUserSet.(*schema.Set).List(), newUserSet.(*schema.Set).List())
		var usersAdded = difference(newUserSet.(*schema.Set).List(), oldUserSet.(*schema.Set).List())

		if len(usersRemoved) > 0 {

			var usersRemovedAsString = GetUsersnamesForUsesysid(tx, usersRemoved)

			if _, err := tx.Exec("ALTER GROUP " + d.Get("group_name").(string) + " DROP USER " + strings.Join(usersRemovedAsString, ", ")); err != nil {
				return err
			}
		}
		if len(usersAdded) > 0 {

			var usersAddedAsString = GetUsersnamesForUsesysid(tx, usersAdded)

			if _, err := tx.Exec("ALTER GROUP " + d.Get("group_name").(string) + " ADD USER " + strings.Join(usersAddedAsString, ", ")); err != nil {
				return err
			}
		}
	}

	err := readRedshiftGroup(d, tx)

	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	return nil
}

func resourceRedshiftGroupDelete(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*Client).db

	//We need to drop all privileges and default privileges
	rows, schemasError := client.Query("select nspname from pg_namespace")
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
		client.Exec("REVOKE ALL ON ALL TABLES IN SCHEMA " + schemaName + " FROM GROUP " + d.Get("group_name").(string))
		client.Exec("ALTER DEFAULT PRIVILEGES IN SCHEMA " + schemaName + " REVOKE ALL ON TABLES FROM GROUP " + d.Get("group_name").(string) + " CASCADE")
	}

	_, err := client.Exec("DROP GROUP " + d.Get("group_name").(string))

	if err != nil {
		log.Print(err)
		return err
	}

	return nil
}

func resourceRedshiftGroupImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceRedshiftGroupRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

func GetGroupNameForGroupId(q Queryer, grosysid int) (string, error) {

	var name string

	err := q.QueryRow("SELECT groname FROM pg_group WHERE grosysid = $1", grosysid).Scan(&name)
	switch {
	case err == sql.ErrNoRows:
		//Is this a good idea?
		return "", err
	case err != nil:
		return "", err
	}
	return name, nil
}

// Complexity: O(n^2)
// Returns a minus b
// Inspired by https://github.com/juliangruber/go-intersect/blob/master/intersect.go
func difference(a []interface{}, b []interface{}) []interface{} {

	set := make([]interface{}, 0)

	for i := 0; i < len(a); i++ {
		el := a[i].(int)
		if !contains(b, el) {
			set = append(set, el)
		}
	}

	return set
}

func contains(v []interface{}, e interface{}) bool {

	for i := 0; i < len(v); i++ {
		if v[i].(int) == e {
			return true
		}
	}
	return false
}
