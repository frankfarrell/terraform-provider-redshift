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
	sslmode  string
}

type Client struct {
	config Config
	db *sql.DB
}

// New redshift client
func (c *Config) Client() *Client {

	conninfo := fmt.Sprintf("sslmode=%v user=%v password=%v host=%v port=%v dbname=%v",
		c.sslmode,
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

	return &Client{
		config: *c,
		db:     db,
	}
}

//When do we close the connection?
