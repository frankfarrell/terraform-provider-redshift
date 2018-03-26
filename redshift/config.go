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
	port     int
	database string
}

// New redshift client
func (c *Config) Client() *sql.DB {

	conninfo := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v",
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
