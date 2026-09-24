// Package store is the SQLite persistence layer.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	// busy_timeout: wait for a lock instead of failing when writes overlap.
	// synchronous=NORMAL is safe with WAL and much faster than FULL.
	db, err := sql.Open("sqlite3", "file:"+path+"?mode=rwc&_txlock=immediate&_journal=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Schema changes, in order. Never edit one that has shipped, add a new one.
var migrations = []string{
	`
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
	`,
	// Characters. Players choose one of up to 4 to play. The daily wordle is
	// still once per account, but records which character played it. Deleted
	// characters keep their row so the leaderboard keeps their results; only
	// live ones hold a slot.
	`
	CREATE TABLE Character (
		character_id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id    INTEGER NOT NULL REFERENCES Player(player_id),
		slot         INTEGER NOT NULL CHECK (slot BETWEEN 0 AND 3),
		name         TEXT NOT NULL,
		class        TEXT NOT NULL,
		deleted      INTEGER NOT NULL DEFAULT 0
	);
	CREATE UNIQUE INDEX character_slot ON Character(player_id, slot) WHERE deleted = 0;
	CREATE INDEX character_player ON Character(player_id);

	-- Players who already played get a knight named after them to keep their results
	INSERT INTO Character (player_id, slot, name, class)
	SELECT player_id, 0, username, 'knight' FROM Player
	WHERE player_id IN (SELECT player_id FROM Wordle);

	CREATE TABLE Wordle2 (
		wordle_id    INTEGER PRIMARY KEY AUTOINCREMENT,
		date         TEXT NOT NULL,
		win          INTEGER NOT NULL DEFAULT 0,
		seconds      REAL NOT NULL DEFAULT 0,
		guessCount   INTEGER NOT NULL,
		player_id    INTEGER NOT NULL REFERENCES Player(player_id),
		character_id INTEGER NOT NULL REFERENCES Character(character_id),
		UNIQUE (player_id, date)
	);
	INSERT INTO Wordle2 (wordle_id, date, win, seconds, guessCount, player_id, character_id)
	SELECT w.wordle_id, w.date, w.win, w.seconds, w.guessCount, w.player_id, c.character_id
	FROM Wordle w INNER JOIN Character c ON c.player_id = w.player_id;
	DROP TABLE Wordle;
	ALTER TABLE Wordle2 RENAME TO Wordle;
	CREATE INDEX wordle_date ON Wordle(date, win);
	`,
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}
	var version int
	err := s.db.QueryRow(`SELECT version FROM schema_version`).Scan(&version)
	if err == sql.ErrNoRows {
		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (0)`); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(`UPDATE schema_version SET version = ?`, i+1); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
