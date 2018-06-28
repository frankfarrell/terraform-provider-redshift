
variable "url" {
  default = "localhost"
}
variable "username" {}
variable "password" {}
variable "database_primary" {}
variable "database_test" {
  default = "testdb"
}

provider redshift {
  "url" = "${var.url}",
  user = "${var.username}",
  password = "${var.password}",
  database = "${var.database_primary}"
  sslmode = "disable"
}

resource "redshift_user" "testuser"{
  "username" = "testusernew",
  "password" = "Testpass123"
  "connection_limit" = "4"
  "createdb" = true
}

resource "redshift_user" "testuser2"{
  "username" = "testuser8",
  "password" = "Testpass123"
  "connection_limit" = "1"
  "createdb" = true
}


resource "redshift_group" "testgroup" {
  "group_name" = "testgroup"
  "users" = ["${redshift_user.testuser.id}", "${redshift_user.testuser2.id}"]
}

resource "redshift_schema" "testschema" {
  "schema_name" = "testschemax",
  "cascade_on_delete" = true
}

resource "redshift_group_schema_privilege" "testgroup_testchema_privileges" {
  "schema_id" = "${redshift_schema.testschema.id}"
  "group_id" = "${redshift_group.testgroup.id}"
  "select" = true
  "insert" = true
  "update" = true
}


#resource "redshift_database" "testdb" {
#  "database_name" = "${var.database_test}",
#  "owner" ="${redshift_user.testuser2.id}",
#  "connection_limit" = "4"
#}