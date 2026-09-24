package server_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/gorilla/websocket"

	"nytrpg/internal/gameday"
	"nytrpg/internal/protocol"
	"nytrpg/internal/puzzles/wordle"
	"nytrpg/internal/testkit"
)

type characterList struct {
	Slots   []*protocol.CharacterInfo `json:"slots"`
	Classes []protocol.ClassInfo      `json:"classes"`
}

func TestCharactersAPI(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	acct := ts.Login(t, "alice") // has a knight in slot 0

	for _, token := range []string{"", "garbage"} {
		if code := ts.Request(t, http.MethodGet, "/characters", token, nil, nil); code != http.StatusUnauthorized {
			t.Errorf("token %q: want 401, got %d", token, code)
		}
	}

	var list characterList
	if code := ts.Request(t, http.MethodGet, "/characters", acct.Token, nil, &list); code != http.StatusOK {
		t.Fatalf("list: %d", code)
	}
	if len(list.Slots) != 4 || list.Slots[0] == nil || list.Slots[0].Class != "knight" || list.Slots[1] != nil {
		t.Fatalf("want the knight in slot 0 and 3 empty slots: %+v", list.Slots)
	}
	if len(list.Classes) != 4 || list.Classes[1].ID != "wizard" || len(list.Classes[1].Abilities) != 5 {
		t.Fatalf("want 4 classes with 5 ability slots each: %+v", list.Classes)
	}

	wiz := ts.CreateCharacter(t, acct, 1, "  Merlin  ", "wizard")
	if wiz.Name != "Merlin" || wiz.Slot != 1 || wiz.Class != "wizard" || wiz.ID == 0 {
		t.Fatalf("created %+v", wiz)
	}

	bad := []struct {
		body map[string]any
		want int
	}{
		{map[string]any{"slot": 2, "name": "", "class": "wizard"}, http.StatusBadRequest},
		{map[string]any{"slot": 2, "name": "   ", "class": "wizard"}, http.StatusBadRequest},
		{map[string]any{"slot": 2, "name": strings.Repeat("x", 21), "class": "wizard"}, http.StatusBadRequest},
		{map[string]any{"slot": 2, "name": "Bard", "class": "bard"}, http.StatusBadRequest},
		{map[string]any{"slot": 4, "name": "Far", "class": "rogue"}, http.StatusBadRequest},
		{map[string]any{"slot": -1, "name": "Neg", "class": "rogue"}, http.StatusBadRequest},
		{map[string]any{"slot": 1, "name": "Again", "class": "rogue"}, http.StatusConflict},
	}
	for _, b := range bad {
		if code := ts.Request(t, http.MethodPost, "/characters", acct.Token, b.body, nil); code != b.want {
			t.Errorf("%+v: want %d, got %d", b.body, b.want, code)
		}
	}

	// Someone else can't delete it
	bob := ts.Login(t, "bob")
	if code := ts.Request(t, http.MethodDelete, "/characters?id="+strconv.Itoa(wiz.ID), bob.Token, nil, nil); code != http.StatusNotFound {
		t.Fatalf("bob deleting alice's character: want 404, got %d", code)
	}
	if code := ts.Request(t, http.MethodDelete, "/characters?id=nope", acct.Token, nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad id: want 400, got %d", code)
	}
	if code := ts.Request(t, http.MethodDelete, "/characters?id="+strconv.Itoa(wiz.ID), acct.Token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", code)
	}
	ts.Request(t, http.MethodGet, "/characters", acct.Token, nil, &list)
	if list.Slots[1] != nil {
		t.Fatalf("deleted character still in its slot: %+v", list.Slots[1])
	}
	ts.CreateCharacter(t, acct, 1, "Merlin II", "wizard") // the slot is free again

	if code := ts.Request(t, http.MethodPut, "/characters", acct.Token, nil, nil); code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT: want 405, got %d", code)
	}
}

func TestWebsocketNeedsYourOwnLiveCharacter(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	alice := ts.Login(t, "alice")
	bob := ts.Login(t, "bob")
	gone := ts.CreateCharacter(t, alice, 1, "Gone", "rogue")
	ts.Request(t, http.MethodDelete, "/characters?id="+strconv.Itoa(gone.ID), alice.Token, nil, nil)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?token=" + alice.Token
	for _, q := range []string{"", "&character=", "&character=abc"} {
		_, resp, err := websocket.DefaultDialer.Dial(wsURL+q, nil)
		if err == nil || resp == nil || resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: want 400, got %v %v", q, resp, err)
		}
	}
	for name, id := range map[string]int{"bob's": bob.CharacterID, "deleted": gone.ID, "unknown": 99999} {
		if _, status, err := ts.DialRaw(alice.Token, id); err == nil || status != http.StatusForbidden {
			t.Errorf("%s character: want 403, got %d %v", name, status, err)
		}
	}
	// None of that left alice marked online
	ts.Dial(t, alice)
}

func TestPlayersSeeCharacterAndClass(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	acct := ts.Login(t, "alice")
	acct.CharacterID = ts.CreateCharacter(t, acct, 2, "Merlin", "wizard").ID
	a := ts.Dial(t, acct)

	w := a.Welcome
	if w.Character != (protocol.CharacterInfo{ID: acct.CharacterID, Slot: 2, Name: "Merlin", Class: "wizard"}) {
		t.Fatalf("welcome should carry the character: %+v", w.Character)
	}
	if len(w.Classes) != 4 {
		t.Fatalf("welcome should list the classes: %+v", w.Classes)
	}
	for _, c := range w.Classes {
		if len(c.Abilities) != 5 {
			t.Fatalf("%s should have 5 ability slots: %+v", c.ID, c.Abilities)
		}
	}

	b := ts.Connect(t, "bob")
	view := b.WatchWorld(testkit.Timeout, func(v testkit.View) bool { _, ok := v.Spawns[w.EntityID]; return ok })
	s := view.Spawns[w.EntityID]
	if s.Name != "alice" || s.Char != "Merlin" || s.Class != "wizard" {
		t.Fatalf("bob should see alice's username, character and class: %+v", s)
	}
}

// Once per account a day, whichever character plays
func TestWordleIsPerAccount(t *testing.T) {
	t.Parallel()
	ts := testkit.NewServer(t)
	acct := ts.Login(t, "alice")
	answer := wordle.LoadWords().For(gameday.Today())

	play := func(c *testkit.Client) protocol.WordleResume {
		c.Send(t, protocol.ClientWordleStart, nil)
		return testkit.Expect[protocol.WordleResume](t, c, protocol.ServerWordleResume)
	}

	knight := ts.Dial(t, acct)
	if r := play(knight); r.Played {
		t.Fatalf("fresh knight: %+v", r)
	}
	knight.Send(t, protocol.ClientWordleGuess, protocol.WordleReq{Guess: answer})
	if res := testkit.Expect[protocol.WordleRes](t, knight, protocol.ServerWordleResult); res.Status != protocol.WordleWin {
		t.Fatalf("winning guess: %+v", res)
	}
	if r := play(knight); !r.Played {
		t.Fatalf("knight already played: %+v", r)
	}
	knight.Close()
	knight.WaitClosed(t)

	alt := acct
	alt.CharacterID = ts.CreateCharacter(t, acct, 1, "Tuck", "cleric").ID
	cleric := ts.Dial(t, alt)
	if r := play(cleric); !r.Played {
		t.Fatalf("the account already played today, the cleric can't play again: %+v", r)
	}
	cleric.Send(t, protocol.ClientWordleGuess, protocol.WordleReq{Guess: answer})
	if res := testkit.Expect[protocol.WordleRes](t, cleric, protocol.ServerWordleResult); res.Valid {
		t.Fatalf("the cleric's guess was accepted: %+v", res)
	}
	// Nor does deleting the character that played free up the day
	cleric.Close()
	cleric.WaitClosed(t)
	ts.Request(t, http.MethodDelete, "/characters?id="+strconv.Itoa(acct.CharacterID), acct.Token, nil, nil)
	cleric = ts.Dial(t, alt)
	if r := play(cleric); !r.Played {
		t.Fatalf("deleting the knight reset today's wordle: %+v", r)
	}

	var lb struct {
		Rows []struct {
			Uname       string `json:"uname"`
			CharacterID int    `json:"characterId"`
			Char        string `json:"char"`
			Class       string `json:"class"`
		} `json:"rows"`
	}
	ts.GetJSON(t, "/leaderboard", &lb)
	if len(lb.Rows) != 1 || lb.Rows[0].Uname != "alice" || lb.Rows[0].Char != "alice" || lb.Rows[0].Class != "knight" || lb.Rows[0].CharacterID != acct.CharacterID {
		t.Fatalf("leaderboard should show the character that won: %+v", lb.Rows)
	}
}
