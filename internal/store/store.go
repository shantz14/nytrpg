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
	db, err := sql.Open("sqlite3", "file:"+path+"?mode=rwc&_txlock=immediate&_journal=WAL")
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
