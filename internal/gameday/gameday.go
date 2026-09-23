// Package gameday defines what "today" means for daily puzzles.
package gameday

import "time"

// The game day runs on fixed UTC-7 (MST, no DST).
var Zone = time.FixedZone("MST", -7*60*60)

const Layout = "2006-01-02"

// Returns the game day of t as YYYY-MM-DD
func Of(t time.Time) string {
	return t.In(Zone).Format(Layout)
}

// Returns the current game day as YYYY-MM-DD
func Today() string {
	return Of(time.Now())
}

// True if s is a YYYY-MM-DD date
func Valid(s string) bool {
	_, err := time.Parse(Layout, s)
	return err == nil
}
