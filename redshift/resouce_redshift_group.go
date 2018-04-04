package redshift

import (
	"github.com/hashicorp/terraform/helper/schema"
	"database/sql"
	"log"
	"strings"
	"strconv"
	"reflect"
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
				Set:      schema.HashString,
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
	client := meta.(*sql.DB)

	var createStatement string = "create group " + d.Get("group_name").(string)

	if v, ok := d.GetOk("users"); ok {

		var usernames = GetUsersnameForUsesysid(client, v.(*schema.Set).List())

		createStatement += " WITH USERS " + strings.Join(usernames, ", ")
	}

	if _, err := client.Exec(createStatement); err != nil {
		log.Fatal(err)
		return err
	}

	var grosysid string
	err := client.QueryRow("SELECT grosysid FROM pg_group WHERE groname = $1", d.Get("group_name").(string)).Scan(&grosysid)

	if err != nil {
		log.Fatal(err)
		return err
	}

	d.SetId(grosysid)

	return resourceRedshiftGroupRead(d, meta)
}

func resourceRedshiftGroupRead(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*sql.DB)

	var (
		groupname 	string
		users 		sql.NullString
	)

	err := client.QueryRow("select groname, grolist from pg_group grosysid = $1", d.Id()).Scan(&groupname, &users)

	if err != nil {
		log.Fatal(err)
		return err
	}

	d.Set("group_name", groupname)

	//Notes on postgres array types https://gist.github.com/adharris/4163702, eg startying with underscore _int4

	if users.Valid {
		var userIdsAsString = strings.Split(users.String[1:len(users.String)-1], ',')
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

	client := meta.(*sql.DB)

	if d.HasChange("group_name") {

		oldName, newName := d.GetChange("group_name")
		alterDatabaseNameQuery := "ALTER GROUP " + oldName.(string) + " RENAME TO " + newName.(string)

		if _, err := client.Exec(alterDatabaseNameQuery); err != nil {
			return err
		}
	}

	if d.HasChange("users") {

		oldUserSet, newUserSet := d.GetChange("users")

		var usersRemoved = difference(oldUserSet.(*schema.Set).List(), newUserSet.(*schema.Set).List())
		var usersAdded = difference(newUserSet.(*schema.Set).List(), oldUserSet.(*schema.Set).List())

		if len(usersRemoved) > 0 {

			var usersRemovedAsString = GetUsersnameForUsesysid(client, usersRemoved)

			if _, err := client.Exec("ALTER GROUP " + d.Get("group_name").(string) + " DROP USER " +  strings.Join(usersRemovedAsString, ", ")); err != nil {
				return err
			}
		}
		if len(usersAdded) > 0 {

			var usersAddedAsString = GetUsersnameForUsesysid(client, usersAdded)

			if _, err := client.Exec("ALTER GROUP " + d.Get("group_name").(string) + " ADD USER " +  strings.Join(usersAddedAsString, ", ")); err != nil {
				return err
			}
		}
	}

	return resourceRedshiftGroupRead(d, meta)
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

func GetUsersnameForUsesysid(client *sql.DB, usersIds []int) []string {

	var usernames []string

	var selectUserQuery = "select usename from pg_user_info where usesysid in (" +
		"?" + strings.Repeat(",?", len(usersIds)-1) +
		")"
	rows, err := client.Query(selectUserQuery, usersIds)
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

// Complexity: O(n^2)
// Returns a minus b
// Inspired by https://github.com/juliangruber/go-intersect/blob/master/intersect.go
func difference(a interface{}, b interface{}) interface{} {

	set := make([]interface{}, 0)
	av := reflect.ValueOf(a)

	for i := 0; i < av.Len(); i++ {
		el := av.Index(i).Interface()
		if !contains(b, el) {
			set = append(set, el)
		}
	}

	return set
}

func contains(a interface{}, e interface{}) bool {
	v := reflect.ValueOf(a)

	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == e {
			return true
		}
	}
	return false
}