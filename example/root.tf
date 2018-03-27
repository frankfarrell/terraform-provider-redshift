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