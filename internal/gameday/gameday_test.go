package gameday

import (
	"testing"
	"time"
)

func TestDayFlipsAtMidnightUTCMinus7(t *testing.T) {
	cases := []struct {
		utc  string
		want string
	}{
		{"2026-09-24T06:59:59Z", "2026-09-23"}, // 23:59:59 game time
		{"2026-09-24T07:00:00Z", "2026-09-24"}, // midnight game time
		// No DST: the same offset in winter
		{"2026-01-15T06:59:59Z", "2026-01-14"},
		{"2026-01-15T07:00:00Z", "2026-01-15"},
	}
	for _, c := range cases {
		at, _ := time.Parse(time.RFC3339, c.utc)
		if got := Of(at); got != c.want {
			t.Errorf("Of(%s) = %s, want %s", c.utc, got, c.want)
		}
	}
}

func TestValid(t *testing.T) {
	for s, want := range map[string]bool{"2026-09-23": true, "2026-9-23": false, "2026-02-30": false, "": false, "nope": false} {
		if Valid(s) != want {
			t.Errorf("Valid(%q) != %v", s, want)
		}
	}
}
