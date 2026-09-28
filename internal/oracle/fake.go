package oracle

import (
	"context"
	"fmt"

	"nytrpg/internal/protocol"
)

// Answers every prayer at once with a canned riddle, without the network: for
// tests, and PRAYER_FAKE=1 in end-to-end tests. Err makes every prayer fail.
type Fake struct {
	Err error
}

func (f Fake) Pray(ctx context.Context, p Prayer) (Answer, error) {
	if f.Err != nil {
		return Answer{}, f.Err
	}
	god := Gods[0]
	if p.Kind == protocol.PrayerMinor {
		return Answer{God: god, Text: fmt.Sprintf("Seek the %s place, mortal, where %c waits.", ordinal(p.Pos+1), p.Word[p.Pos])}, nil
	}
	return Answer{God: god, Text: fmt.Sprintf("Five marks, mortal, and the first of them is %c.", p.Word[0])}, nil
}
