package redshift


import (
	_ "github.com/lib/pq"
	"database/sql"
	"log"
	"fmt"
)

// Config holds API and APP keys to authenticate to Datadog.
type Config struct {
	url string
	user string
	password string
	port int
	database string
}

// New redshift client
func (c *Config) Client() *sql.DB {

	conninfo, err :=
		fmt.Printf("%v:%v@tcp(%v:%v)/%v",
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