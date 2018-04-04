provider redshift {
  "url" = "localhost",
  user = "testroot",
  password = "Rootpass123",
  database = "dev"
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


//resource "redshift_group" "testgroup" {
//  "group_name" = "testgroup"
//  "users" = [101]//"${redshift_user.testuser.usesysid}"]
//}
//
resource "redshift_database" "testdb" {
  "database_name" = "testdb",
  "owner" ="${redshift_user.testuser2.id}",
  "connection_limit" = "4"
}