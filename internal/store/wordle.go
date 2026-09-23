package store

type WordleResult struct {
	Date       string
	Win        bool
	Seconds    float64
	GuessCount int
	PlayerID   int
}

type LeaderboardRow struct {
	Place   int     `json:"place"`
	Uname   string  `json:"uname"`
	Guesses int     `json:"guesses"`
	Time    float64 `json:"time"`
}

func (s *Store) PlayedWordleOn(pid int, date string) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM Wordle WHERE player_id = ? AND date = ?;", pid, date).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) InsertWordle(r WordleResult) error {
	win := 0
	if r.Win {
		win = 1
	}
	_, err := s.db.Exec(`
	INSERT INTO Wordle (date, win, seconds, guessCount, player_id)
	VALUES (?, ?, ?, ?, ?);
	`, r.Date, win, r.Seconds, r.GuessCount, r.PlayerID)
	return err
}

// One page of winners for a date, and the total number of winners
func (s *Store) WordleLeaderboard(date string, limit int, offset int) ([]LeaderboardRow, int, error) {
	result := make([]LeaderboardRow, 0, limit)

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM Wordle WHERE date = ? AND win = 1;", date).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
	SELECT p.username, w.guessCount, w.seconds FROM Wordle w
	INNER JOIN Player p ON w.player_id = p.player_id
	WHERE w.date = ? AND w.win = 1
	ORDER BY w.guessCount ASC, w.seconds ASC
	LIMIT ? OFFSET ?;
	`, date, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	place := offset + 1
	for rows.Next() {
		var row LeaderboardRow
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
