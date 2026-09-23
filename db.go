package main

import (
	"database/sql"
	"errors"
	"log"
	"os"

	"github.com/mattn/go-sqlite3"
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
	os.MkdirAll("db", 0755)
	db, err := sql.Open("sqlite3",
		"file:./db/nytrpg.db?mode=rwc&_txlock=immediate&_journal=WAL")
	if err != nil {
		log.Fatal(err)
	}
	c.pool = db
	c.migrate()
}

func (c *Connection) migrate() {
	_, err := c.pool.Exec(`
		CREATE TABLE IF NOT EXISTS Player (
			player_id INTEGER PRIMARY KEY AUTOINCREMENT,
			username  TEXT NOT NULL UNIQUE,
			password  TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS Wordle (
			wordle_id  INTEGER PRIMARY KEY AUTOINCREMENT,
			date       TEXT NOT NULL,
			win        INTEGER NOT NULL DEFAULT 0,
			seconds    REAL NOT NULL DEFAULT 0,
			guessCount INTEGER NOT NULL,
			player_id  INTEGER NOT NULL,
			FOREIGN KEY (player_id) REFERENCES Player(player_id),
			UNIQUE (player_id, date)
		);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

// Returns (row, exists) of the player with the given username
func (c *Connection) getPlayerByUname(uname string) (PlayerRow, bool) {
	var p PlayerRow
	err := c.pool.QueryRow("SELECT player_id, username FROM Player WHERE username = ?", uname).Scan(&p.id, &p.username)
	if err == sql.ErrNoRows {
		return p, false
	}
	if err != nil {
		log.Println("Error getting player by username:", err)
		return p, false
	}
	return p, true
}

// Returns the id, username, and password hash for logging in
func (c *Connection) getPlayerAuth(uname string) (id int, username string, hash string, found bool, err error) {
	err = c.pool.QueryRow("SELECT player_id, username, password FROM Player WHERE username = ?", uname).Scan(&id, &username, &hash)
	if err == sql.ErrNoRows {
		return 0, "", "", false, nil
	}
	if err != nil {
		return 0, "", "", false, err
	}
	return id, username, hash, true, nil
}

// Returns taken = true if the username already exists
func (c *Connection) insertPlayer(uname string, hash string) (taken bool, err error) {
	_, err = c.pool.Exec("INSERT INTO Player (username, password) VALUES (?, ?);", uname, hash)
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return true, nil
	}
	return false, err
}

func (c *Connection) playedOn(pid int, date string) (bool, error) {
	var n int
	err := c.pool.QueryRow("SELECT COUNT(*) FROM Wordle WHERE player_id = ? AND date = ?;", pid, date).Scan(&n)
	if err != nil {
		log.Println("Error checking if player played:", err)
		return false, err
	}
	return n > 0, nil
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

// One page of winners for a date, and the total number of winners
func (c *Connection) getLeaderboard(date string, limit int, offset int) ([]lbRow, int, error) {
	result := make([]lbRow, 0, limit)

	var total int
	err := c.pool.QueryRow("SELECT COUNT(*) FROM Wordle WHERE date = ? AND win = 1;", date).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	sql := `
	SELECT p.username, w.guessCount, w.seconds FROM Wordle w
	INNER JOIN Player p ON w.player_id = p.player_id
	WHERE w.date = ? AND w.win = 1
	ORDER BY w.guessCount ASC, w.seconds ASC
	LIMIT ? OFFSET ?;
	`
	rows, err := c.pool.Query(sql, date, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	place := offset + 1
	for rows.Next() {
		var row lbRow
		if err := rows.Scan(&row.Uname, &row.Guesses, &row.Time); err != nil {
			return nil, 0, err
		}
		row.Place = place
		result = append(result, row)
		place++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}
