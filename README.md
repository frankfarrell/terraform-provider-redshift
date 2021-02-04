# Terraform Redshift Provider

[![Codacy Badge](https://api.codacy.com/project/badge/Grade/076b7e35151040f1802b500f218950d1)](https://www.codacy.com/app/frankfarrell/terraform-provider-redshift?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=frankfarrell/terraform-provider-redshift&amp;utm_campaign=Badge_Grade)
[![Build Status](https://travis-ci.org/frankfarrell/terraform-provider-redshift.svg?branch=master)](https://travis-ci.org/frankfarrell/terraform-provider-redshift)
[![Gitter chat](https://badges.gitter.im/gitterHQ/gitter.svg)](https://gitter.im/terraform-redshift-provider)

Manage Redshift users, groups, privileges, databases and schemas. It runs the SQL queries necessary to manage these (CREATE USER, DELETE DATABASE etc)
in transactions, and also reads the state from the tables that store this state, eg pg_user_info, pg_group etc. The underlying tables are more or less equivalent to the postgres tables, 
but some tables are not accessible in Redshift. 

Currently supports users, groups, schemas and databases. You can set privileges for groups on schemas. Per user schema privileges will be added at a later date. 

Note that schemas are the lowest level of granularity here, tables should be created by some other tool, for instance flyway. 

# Get it:

1. Navigate to the [latest release][latest_release] and download the applicable
   plugin binary.
1. [Add to terraform plugins directory][installing_plugin] installed
1. Run `terraform init` to register the plugin in your project


## Legacy download links (0.0.2)

Download for amd64 (for other architectures and OSes you can build from source as descibed below)
* [Windows](https://github.com/frankfarrell/terraform-provider-redshift/raw/cff73548b/dist/windows/amd64/terraform-provider-redshift_v0.0.2_x4.exe)
* [Linux](https://github.com/frankfarrell/terraform-provider-redshift/raw/cff73548b/dist/linux/amd64/terraform-provider-redshift_v0.0.2_x4)
* [Mac](https://github.com/frankfarrell/terraform-provider-redshift/raw/cff73548b/dist/darwin/amd64/terraform-provider-redshift_v0.0.1_x4)

## Examples:

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
# Create a user
resource "redshift_user" "testuser"{
  "username" = "testusernew" # User name are not immutable. 
  # Terraform can't read passwords, so if the user changes their password it will not be picked up. One caveat is that when the user name is changed, the password is reset to this value
  "password" = "Testpass123" # You can pass an md5 encryted password here by prefixing the hash with md5
  "valid_until" = "2018-10-30" # See below for an example with 'password_disabled'
  "connection_limit" = "4"
  "createdb" = true
  "syslog_access" = "UNRESTRICTED"
  "superuser" = true
}

# Add the user to a new group
resource "redshift_group" "testgroup" {
  "group_name" = "testgroup" # Group names are not immutable
  "users" = ["${redshift_user.testuser.id}"] # A list of user ids as output by terraform (from the pg_user_info table), not a list of usernames (they are not immnutable)
}

# Create a schema
resource "redshift_schema" "testschema" {
  "schema_name" = "testschema", # Schema names are not immutable
  "owner" ="${redshift_user.testuser.id}", # This defaults to the current user (eg as specified in the provider config) if empty
  "cascade_on_delete" = true
}

# Give that group select, insert and references privileges on that schema
resource "redshift_group_schema_privilege" "testgroup_testchema_privileges" {
  "schema_id" = "${redshift_schema.testschema.id}" # Id rather than group name
  "group_id" = "${redshift_group.testgroup.id}" # Id rather than group name
  "select" = true
  "insert" = true
  "update" = false
  "references" = true
  "delete" = false # False values are optional
}
```

You can only create resources in the db configured in the provider block. Since you cannot configure providers with 
the output of resources, if you want to create a db and configure resources you will need to configure it through a `terraform_remote_state` data provider. 
Even if you specifiy the name directly rather than as a variable, since providers are configured before resources you will need to have them in separate projects. 

```
# First file:

resource "redshift_database" "testdb" {
  "database_name" = "testdb", # This isn't immutable
  "owner" ="${redshift_user.testuser.id}",
  "connection_limit" = "4"
}

output "testdb_name" {
  value = "${redshift_database.testdb.database_name}"
}

# Second file: 

data "terraform_remote_state" "redshift" {
  backend = "s3"
  config {
    bucket = "somebucket"
    key = "somekey"
    region = "us-east-1"
  }
}

provider redshift {
  "url" = "localhost",
  user = "testroot",
  password = "Rootpass123",
  database = "${data.terraform_remote_state.redshift.testdb_name}"
}

```

Creating a user who can only connect using IAM Credentials as described [here](https://docs.aws.amazon.com/redshift/latest/mgmt/generating-user-credentials.html)

```
resource "redshift_user" "testuser"{
  "username" = "testusernew",
  "password_disabled" = true # No need to specify a pasword is this is true
  "connection_limit" = "1"
}
```

## Things to note
### Limitations
For authoritative limitations, please see the Redshift documentations.
1) You cannot delete the database you are currently connected to.
2) You cannot set table specific privileges since this provider is table agnostic (for now, if you think it would be feasible to manage tables let me know)
3) On importing a user, it is impossible to read the password (or even the md hash of the password, since Redshift restricts access to pg_shadow)

### I usually connect through an ssh tunnel, what do I do?
The easiest thing is probably to update your hosts file so that the url resolves to localhost

## Contributing:

### Prequisites to development
1. Go installed
2. Terraform installed locally

### Building
Run `make dist` to generate binaries for the supported os/architectures. This
process relies on GNUMake and bash, but you can always fallback to generating
your own binaries with `go build -o your-binary-here`.

Once generated, you can add the binary to your terraform plugins directory to
get it working. (e.g.
terraform.d/linux/amd64/terraform-provider-redshift_vblah) Note that the prefix
of the binary must match, and follow guidelines for [Terraform
directories][installing_plugin]

After installing the plugin you can debug crudely by setting the TF_LOG env
variable to DEBUG. Eg

```
$ TF_LOG=DEBUG terraform apply
```

### Releasing
If you are cutting a new release, update the `VERSION` file to the new release
number prior to running `make release`. You will be prompted for the prior
version to auto-generate a changelog entry. Review the diffs in CHANGELOG.md
before committing.

Generate binaries hr each system by running `make dist`. Once gathered,
add a final tag to mark the github SHA for the release:

```
git tag -m $(cat VERSION) $(cat VERSION)
git push $(cat VERSION)
```

Navigate to the [project
tag](https://github.com/frankfarrell/terraform-provider-redshift/tags) to edit
the release.  Add the compiled binaries and publish the release.


## TODO
1. Database property for Schema
2. Schema privileges on a per user basis
3. Add privileges for languages and functions

[installing_plugin]: https://www.terraform.io/docs/extend/how-terraform-works.html#implied-local-mirror-directories

[latest_release]: https://github.com/frankfarrell/terraform-provider-redshift/releases/tag/0.0.2
