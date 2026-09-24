package store

import (
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

// Character slots per account
const CharacterSlots = 4

type Character struct {
	ID       int
	PlayerID int
	Slot     int
	Name     string
	// A classes.ID
	Class   string
	Deleted bool
}

// The player's live characters, by slot
func (s *Store) Characters(pid int) ([]Character, error) {
	rows, err := s.db.Query(`
	SELECT character_id, player_id, slot, name, class FROM Character
	WHERE player_id = ? AND deleted = 0 ORDER BY slot;
	`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chars []Character
	for rows.Next() {
		var c Character
		if err := rows.Scan(&c.ID, &c.PlayerID, &c.Slot, &c.Name, &c.Class); err != nil {
			return nil, err
		}
		chars = append(chars, c)
	}
	return chars, rows.Err()
}

// Any character, including deleted ones
func (s *Store) CharacterByID(id int) (Character, bool, error) {
	var c Character
	err := s.db.QueryRow(`
	SELECT character_id, player_id, slot, name, class, deleted FROM Character WHERE character_id = ?;
	`, id).Scan(&c.ID, &c.PlayerID, &c.Slot, &c.Name, &c.Class, &c.Deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	return c, true, nil
}

// Returns taken = true if the player already has a character in that slot
func (s *Store) InsertCharacter(pid, slot int, name, class string) (c Character, taken bool, err error) {
	res, err := s.db.Exec("INSERT INTO Character (player_id, slot, name, class) VALUES (?, ?, ?, ?);", pid, slot, name, class)
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) && sqlErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return c, true, nil
	}
	if err != nil {
		return c, false, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return c, false, err
	}
	return Character{ID: int(id), PlayerID: pid, Slot: slot, Name: name, Class: class}, false, nil
}

// Deletes one of the player's characters, freeing its slot. found is false if
// it doesn't exist, isn't theirs, or is already deleted.
func (s *Store) DeleteCharacter(pid, id int) (found bool, err error) {
	res, err := s.db.Exec("UPDATE Character SET deleted = 1 WHERE character_id = ? AND player_id = ? AND deleted = 0;", id, pid)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
