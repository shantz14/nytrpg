package main

import (
	"database/sql"
	"log"
	"time"

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
				"file:./nytrpg.db?mode=rw&_txlock=immediate&_journal=WAL")
	if err != nil {
		log.Fatal(err)
	}
	c.pool = db
}

// Returns (row, exists) of first player with given id in a PlayerRow struct
func (c *Connection) getPlayerById(id int) (PlayerRow, bool) {
	var p PlayerRow
	rows, err := c.pool.Query("select player_id, username from Player where player_id = ?", id)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
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

func (c *Connection) getPlayerByUname(uname string) (PlayerRow, bool) {
	var p PlayerRow
	rows, err := c.pool.Query("select player_id, username from Player where username = ?", uname)
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

func (c *Connection) playedToday(pid int) bool {
	currentTime := time.Now()
	date := currentTime.Format("2006-01-02")
	sql := `
	SELECT * FROM Wordle
	WHERE player_id = ? AND date = ?;
	`
	rows, err := c.pool.Query(sql, pid, date)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		return true
	}
	return false
}

func (c *Connection) insertWordle(date string, win bool, seconds float32, guessCount int, pid int) {
	sql := `
	INSERT INTO Wordle (date, win, seconds, guessCount, player_id)
	VALUES (?, ?, ?, ?, ?);
	`
	wini := 0
	if win {
		wini = 1
	}
	_, err := c.pool.Exec(sql, date, wini, seconds, guessCount, pid)
	if err != nil {
		log.Println("Failed to insert wordle record.", err)
	}
}




