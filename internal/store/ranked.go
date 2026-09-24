package store

import (
	"database/sql"
	"errors"
)

// A character's ranked record, as stored
type Rating struct {
	CharacterID int
	Elo         int
	Peak        int
	Games       int
	Wins        int
	Losses      int
	Draws       int
}

// The character's rating. found is false if it has never played ranked.
func (s *Store) Rating(charID int) (r Rating, found bool, err error) {
	r.CharacterID = charID
	err = s.db.QueryRow(`
	SELECT elo, peak, games, wins, losses, draws FROM Rating WHERE character_id = ?;
	`, charID).Scan(&r.Elo, &r.Peak, &r.Games, &r.Wins, &r.Losses, &r.Draws)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	return r, err == nil, err
}

// A finished ranked duel between characters A and B
type RankedDuel struct {
	PlayedAt int64
	A, B     int
	// A or B's character id, 0 for a draw
	Winner  int
	Reason  int
	ABefore int
	AAfter  int
	BBefore int
	BAfter  int
	// Guesses each had made when it ended
	AGuesses int
	BGuesses int
	Seconds  float64
}

// Saves a ranked duel and both characters' ratings after it, together
func (s *Store) ApplyRankedDuel(d RankedDuel, a, b Rating) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var winner any
	if d.Winner != 0 {
		winner = d.Winner
	}
	_, err = tx.Exec(`
	INSERT INTO RankedDuel (played_at, a_id, b_id, winner_id, reason, a_before, a_after, b_before, b_after, a_guesses, b_guesses, seconds)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`, d.PlayedAt, d.A, d.B, winner, d.Reason, d.ABefore, d.AAfter, d.BBefore, d.BAfter, d.AGuesses, d.BGuesses, d.Seconds)
	if err != nil {
		return err
	}
	for _, r := range []Rating{a, b} {
		_, err = tx.Exec(`
		INSERT INTO Rating (character_id, elo, peak, games, wins, losses, draws) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (character_id) DO UPDATE SET
			elo = excluded.elo, peak = excluded.peak, games = excluded.games,
			wins = excluded.wins, losses = excluded.losses, draws = excluded.draws;
		`, r.CharacterID, r.Elo, r.Peak, r.Games, r.Wins, r.Losses, r.Draws)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// One ranked duel from a character's side
type RankedDuelRow struct {
	Opponent      string
	OpponentClass string
	Won           bool
	Draw          bool
	// Elo gained, negative for a loss
	Change   int
	PlayedAt int64
}

// The character's latest ranked duels, newest first
func (s *Store) RecentRankedDuels(charID, limit int) ([]RankedDuelRow, error) {
	rows, err := s.db.Query(`
	SELECT c.name, c.class, d.winner_id, d.played_at,
		CASE WHEN d.a_id = ?1 THEN d.a_after - d.a_before ELSE d.b_after - d.b_before END
	FROM RankedDuel d
	JOIN Character c ON c.character_id = CASE WHEN d.a_id = ?1 THEN d.b_id ELSE d.a_id END
	WHERE d.a_id = ?1 OR d.b_id = ?1
	ORDER BY d.played_at DESC, d.ranked_duel_id DESC
	LIMIT ?2;
	`, charID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RankedDuelRow
	for rows.Next() {
		var r RankedDuelRow
		var winner sql.NullInt64
		if err := rows.Scan(&r.Opponent, &r.OpponentClass, &winner, &r.PlayedAt, &r.Change); err != nil {
			return nil, err
		}
		r.Draw = !winner.Valid
		r.Won = winner.Valid && int(winner.Int64) == charID
		out = append(out, r)
	}
	return out, rows.Err()
}
