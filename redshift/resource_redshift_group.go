package redshift

import (
	"database/sql"
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"log"
	"strconv"
	"strings"
	"time"
)

//name and list of users
//https://docs.aws.amazon.com/redshift/latest/dg/r_CREATE_DATABASE.html
//https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_DATABASE.html
//https://docs.aws.amazon.com/redshift/latest/dg/r_ALTER_DATABASE.html

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
	client := meta.(*sql.DB)

	var name string

	err := client.QueryRow("SELECT groname FROM pg_group WHERE grosysid = $1", d.Id()).Scan(&name)
	if err != nil {
		return false, err
	}
	return true, nil
}

func resourceRedshiftGroupCreate(d *schema.ResourceData, meta interface{}) error {
	redshiftClient := meta.(*sql.DB)

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
		log.Fatal(err)
		return err
	}

	log.Print("Database created succesfully, reading grosyid from pg_group")

	//The changes do not propagate instantly
	time.Sleep(5 * time.Second)

	var grosysid int
	err := tx.QueryRow("SELECT grosysid FROM pg_group WHERE groname = $1", d.Get("group_name").(string)).Scan(&grosysid)
	if err != nil {
		log.Fatal(err)
		return err
	}

	log.Printf("grosysid is %s", strconv.Itoa(grosysid))

	d.SetId(strconv.Itoa(grosysid))

	return resourceRedshiftGroupRead(d, meta)

	readErr := readRedshiftGroup(d, tx)

	if readErr == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return readErr
	}
}

func resourceRedshiftGroupRead(d *schema.ResourceData, meta interface{}) error {

	redshiftClient := meta.(*sql.DB)

	tx, txErr := redshiftClient.Begin()

	if txErr != nil {
		panic(txErr)
	}

	err := readRedshiftGroup(d, tx)

	if err == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return err
	}
}

func readRedshiftGroup(d *schema.ResourceData, tx *sql.Tx) error {
	var (
		groupname 	string
		users 		sql.NullString
	)

	err := tx.QueryRow("select groname, grolist from pg_group where grosysid = $1", d.Id()).Scan(&groupname, &users)

	if err != nil {
		log.Fatal(err)
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

	redshiftClient := meta.(*sql.DB)
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

			if _, err := tx.Exec("ALTER GROUP " + d.Get("group_name").(string) + " DROP USER " +  strings.Join(usersRemovedAsString, ", ")); err != nil {
				return err
			}
		}
		if len(usersAdded) > 0 {

			var usersAddedAsString = GetUsersnamesForUsesysid(tx, usersAdded)

			if _, err := tx.Exec("ALTER GROUP " + d.Get("group_name").(string) + " ADD USER " +  strings.Join(usersAddedAsString, ", ")); err != nil {
				return err
			}
		}
	}

	err := readRedshiftGroup(d, tx)

	if err == nil {
		tx.Commit()
		return nil
	} else {
		tx.Rollback()
		return err
	}
}

func resourceRedshiftGroupDelete(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*sql.DB)

	_, err := client.Exec("GROP GROUP " + d.Get("group_name").(string))

	if err != nil {
		log.Fatal(err)
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
