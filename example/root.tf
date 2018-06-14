
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
  "connection_limit" = "4"
}


resource "redshift_group" "testgroup" {
  "group_name" = "testgroup"
  "users" = ["${redshift_user.testuser.id}"]
}

resource "redshift_database" "testdb" {
  "database_name" = "${var.database_test}",
  "owner" ="${redshift_user.testuser2.id}",
  "connection_limit" = "4"
}

provider redshift {
  alias = "test"
  "url" = "${var.url}",
  user = "${var.username}",
  password = "${var.password}",
  database = "${var.database_test}"
  sslmode = "disable"
}

resource "redshift_user" "testusernew2"{
  provider = "redshift.test"
  "username" = "testusernew2",
  "password" = "Testpass123"
  "connection_limit" = "3"
  "createdb" = false
  "valid_until" = "2018-07-10"
}