package redshift

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"log"
)

// Config holds API and APP keys to authenticate to Datadog.
type Config struct {
	url      string
	user     string
	password string
	port     string
	database string
}

// New redshift client
func (c *Config) Client() *sql.DB {

	conninfo := fmt.Sprintf("sslmode=disable user=%v password=%v host=%v port=%v dbname=%v",
		c.user,
		c.password,
		c.url,
		c.port,
		c.database)

	db, err := sql.Open("postgres", conninfo)

	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	return db
}

//When do we close the connection?
