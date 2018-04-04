# Terraform Redshift Provider

Manage redshift users, groups, privileges, databases and schemas. 

Currently only supports users, groups and databses. 

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
```

Creating a user who can only connect using IAM Credentials as described [here](https://docs.aws.amazon.com/redshift/latest/mgmt/generating-user-credentials.html)

```
resource "redshift_user" "testuser"{
  "username" = "testusernew",
  "password_disabled" = true
  "connection_limit" = "1"
}
```

## Prequisites
1. Go installed
2. Terraform installed locally

## Building: 
1. Run `go build -o terraform-provider-redshift_v0.0.1_x4.exe`. You will need to tweak this with GOOS and GOARCH if you are planning to build it for different OSes and architectures
2. Add to terraform plugins directory: https://www.terraform.io/docs/configuration/providers.html#third-party-plugins

## I usually connect through an ssh tunnel, what do I do?
The easiest thing is probably to update your hosts file so that the url resolves to localhost

## TODO 
1. All the other resources! 
2. Port this to postgres
3. Rollback on failure by calling Delete
4. Handle cascading deletes properly