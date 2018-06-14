# Terraform Redshift Provider

Manage Redshift users, groups, privileges, databases and schemas. It runs the SQL queries necessary to manage these (CREATE USER, DELETE DATABASE etc)
in transactions, and also reads the state from the tables that store this state, eg pg_user_info, pg_group etc. The underlying tables are more or less equivalent to the postgres tables, 
but some tables are not accessible in Redshift. 

Currently only supports users, groups and databases. Privileges and schemas coming soon! 

## Limitations
For authoritative limitations, please see the Redshift documentations. 
1) You cannot delete the database you are currently connected to. 
2) You cannot set table specific privileges since this provider is table agnostic (for now, if you think it would be feasible to manage tables let me know)
3) On importing a user, it is impossible to read the password (or even the md hash of the password, since Redshift restricts access to pg_shadow)

## Example: 

Provider configuration
```
provider redshift {
  "url" = "localhost",
  user = "testroot",
  password = "Rootpass123",
  database = "dev"
}
```

Creating an admin user who is in a group and who owns a new database, with a password that expires
```
resource "redshift_user" "testuser"{
  "username" = "testusernew",
  "password" = "Testpass123"
  "valid_until" = "2018-10-30" 
  "connection_limit" = "4"
  "createdb" = true
  "syslog_access" = "UNRESTICTED"
  "superuser" = true
}

resource "redshift_group" "testgroup" {
  "group_name" = "testgroup"
  "users" = ["${redshift_user.testuser.id}"]
}

resource "redshift_database" "testdb" {
  "database_name" = "testdb",
  "owner" ="${redshift_user.testuser.id}",
  "connection_limit" = "4"
}

resource "redshift_schema" "testschema" {
  "schema_name" = "testschema",
  "owner" ="${redshift_user.testuser.id}",
  "cascade_on_delete" = true
}
```

Creating a user who can only connect using IAM Credentials as described [here](https://docs.aws.amazon.com/redshift/latest/mgmt/generating-user-credentials.html)

```
resource "redshift_user" "testuser"{
  "username" = "testusernew",
  "password_disabled" = true
  "connection_limit" = "1"
}
```

## Prequisites to development
1. Go installed
2. Terraform installed locally

## Building: 
1. Run `go build -o terraform-provider-redshift_v0.0.1_x4.exe`. You will need to tweak this with GOOS and GOARCH if you are planning to build it for different OSes and architectures
2. Add to terraform plugins directory: https://www.terraform.io/docs/configuration/providers.html#third-party-plugins

## I usually connect through an ssh tunnel, what do I do?
The easiest thing is probably to update your hosts file so that the url resolves to localhost

## TODO 
1. Database property for Schema
2. Create and usage privileges for schemas