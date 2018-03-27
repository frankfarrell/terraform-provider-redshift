# Terraform Redshift Provider

Manage redshift users, groups, privileges, databases and schemas. 

Currently only supports users, its a WIP

## Example: 
```
provider redshift {
  "url" = "localhost",
  user = "testroot",
  password = "Rootpass123",
  database = "dev"
}

resource "redshift_user" "testuser"{
  "username" = "testuser",
  password = "Testpass123"
  connection_limit = "4"
  createdb = true
}
```

## Prequisites
1. Go installed
2. Terraform installed locally

## Building: 
1. Run `go build -o terraform-provider-redshift_v0.0.1_x4.exe`. You will need to tweak this with 
2. Add to terraform plugins directory: https://www.terraform.io/docs/configuration/providers.html#third-party-plugins

## I usually connect through an ssh tunnel, what do I do?
The easiest thing is probably to update your hosts file so that the url resolves to localhost

## TODO 
1. All the other resources! 
1. Port this to postgres