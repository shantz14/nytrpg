package store

import (
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

type Player struct {
	ID       int
	Username string
}

// Returns (player, found, err) for the given username
func (s *Store) PlayerByUsername(uname string) (Player, bool, error) {
	var p Player
	err := s.db.QueryRow("SELECT player_id, username FROM Player WHERE username = ?", uname).Scan(&p.ID, &p.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	if err != nil {
		return p, false, err
	}
	return p, true, nil
}

// Returns the player and their password hash for logging in
func (s *Store) PlayerAuth(uname string) (p Player, hash string, found bool, err error) {
	err = s.db.QueryRow("SELECT player_id, username, password FROM Player WHERE username = ?", uname).Scan(&p.ID, &p.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return p, "", false, nil
	}
	if err != nil {
		return p, "", false, err
	}
	return p, hash, true, nil
}

// Returns taken = true if the username already exists
func (s *Store) InsertPlayer(uname string, hash string) (taken bool, err error) {
	_, err = s.db.Exec("INSERT INTO Player (username, password) VALUES (?, ?);", uname, hash)
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return true, nil
	}
	return false, err
}
