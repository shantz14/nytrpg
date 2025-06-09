package main

import (
	"log"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Connection struct {
	pool *sql.DB
}

type PlayerRow struct {
	id int
	username string
}

func newConnection() *Connection {
	var conn Connection
	conn.init()
	return &conn
}

func (c *Connection) init() {
	// Create DB connection
	db, err := sql.Open("sqlite3", 
				"file:./nytrpg.db?mode=rw")
	if err != nil {
		log.Fatal(err)
	}
	c.pool = db
}

// Returns (row, exists) of first player with given id in a PlayerRow struct
func (c *Connection) getPlayerById(id int) (PlayerRow, bool) {
	var p PlayerRow
	rows, err := c.pool.Query("select id, username from Player where id = ?", id)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&p.id, &p.username)
		if err != nil {
			log.Fatal(err)
		}
		return p, true
	}
	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}
	return p, false
}
