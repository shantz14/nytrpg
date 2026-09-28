package oracle

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"

	"nytrpg/internal/protocol"
)

func TestPrompts(t *testing.T) {
	g := Gods[3]
	sys := systemPrompt(g, 5)
	if !strings.Contains(sys, g.Name) || !strings.Contains(sys, g.Persona) || !strings.Contains(sys, "5-letter") {
		t.Fatalf("system prompt:\n%s", sys)
	}
	minor := userPrompt(Prayer{Kind: protocol.PrayerMinor, Word: "CRANE", Pos: 2})
	if !strings.Contains(minor, "letter is A") || !strings.Contains(minor, "3rd letter") || strings.Contains(minor, "CRANE") {
		t.Fatalf("a minor prayer names the letter and where, never the word:\n%s", minor)
	}
	if major := userPrompt(Prayer{Kind: protocol.PrayerMajor, Word: "CRANE"}); !strings.Contains(major, "CRANE") {
		t.Fatalf("major prayer:\n%s", major)
	}
}

func TestLeaks(t *testing.T) {
	major := Prayer{Kind: protocol.PrayerMajor, Word: "CRANE"}
	if !leaks(major, "The bird you seek is a crane.") || leaks(major, "A tall bird wades in the reeds.") {
		t.Fatal("major prayers must not name the word")
	}
	if leaks(Prayer{Kind: protocol.PrayerMinor, Word: "CRANE", Pos: 0}, "crane") {
		t.Fatal("minor prayers aren't checked")
	}
}

// A stand-in for the Messages API: answers each request with the next reply
// and records what was asked
type fakeAPI struct {
	mu       sync.Mutex
	replies  []reply
	requests []map[string]any
	betas    []string
}

type reply struct {
	text       string
	stopReason string
	status     int
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	f.requests = append(f.requests, body)
	f.betas = append(f.betas, r.Header.Get("anthropic-beta"))
	rep := f.replies[0]
	if len(f.replies) > 1 {
		f.replies = f.replies[1:]
	}
	w.Header().Set("Content-Type", "application/json")
	if rep.status != 0 {
		w.WriteHeader(rep.status)
		json.NewEncoder(w).Encode(map[string]any{"type": "error", "error": map[string]any{"type": "api_error", "message": "down"}})
		return
	}
	stop := rep.stopReason
	if stop == "" {
		stop = "end_turn"
	}
	json.NewEncoder(w).Encode(map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": Model,
		"content":     []map[string]any{{"type": "text", "text": rep.text}},
		"stop_reason": stop,
		"usage":       map[string]any{"input_tokens": 10, "output_tokens": 10},
	})
}

func testClaude(t *testing.T, replies ...reply) (*Claude, *fakeAPI) {
	t.Helper()
	api := &fakeAPI{replies: replies}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	c := NewClaude(option.WithBaseURL(srv.URL), option.WithAPIKey("test-key"), option.WithMaxRetries(0))
	c.pick = func(int) int { return 2 }
	return c, api
}

func TestClaudeAnswersAsAGod(t *testing.T) {
	c, api := testClaude(t, reply{text: "  Where the tide turns thrice, the letter waits.  "})
	ans, err := c.Pray(context.Background(), Prayer{Kind: protocol.PrayerMinor, Word: "CRANE", Pos: 2})
	if err != nil {
		t.Fatal(err)
	}
	if ans.God.Name != "Oma Sel" || ans.Text != "Where the tide turns thrice, the letter waits." {
		t.Fatalf("answer %+v", ans)
	}
	req := api.requests[0]
	if req["model"] != "claude-opus-5" || req["fallbacks"] != "default" || !strings.Contains(api.betas[0], "server-side-fallback-2026-07-01") {
		t.Fatalf("request %v, betas %q", req, api.betas[0])
	}
	if cfg, _ := req["output_config"].(map[string]any); cfg["effort"] != "low" {
		t.Fatalf("effort %v", req["output_config"])
	}
	system, _ := json.Marshal(req["system"])
	if !strings.Contains(string(system), "Oma Sel") {
		t.Fatalf("the god's persona should be the system prompt: %s", system)
	}
}

func TestClaudeFailures(t *testing.T) {
	major := Prayer{Kind: protocol.PrayerMajor, Word: "CRANE"}
	t.Run("a refusal goes unanswered", func(t *testing.T) {
		c, _ := testClaude(t, reply{text: "", stopReason: "refusal"})
		if _, err := c.Pray(context.Background(), major); !errors.Is(err, ErrUnanswered) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("an API error goes unanswered", func(t *testing.T) {
		c, _ := testClaude(t, reply{status: 500})
		if _, err := c.Pray(context.Background(), major); !errors.Is(err, ErrUnanswered) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("an empty answer goes unanswered", func(t *testing.T) {
		c, _ := testClaude(t, reply{text: "   "})
		if _, err := c.Pray(context.Background(), major); !errors.Is(err, ErrUnanswered) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("a leak is asked again once", func(t *testing.T) {
		c, api := testClaude(t, reply{text: "It's CRANE."}, reply{text: "A long-legged bird."})
		ans, err := c.Pray(context.Background(), major)
		if err != nil || ans.Text != "A long-legged bird." || len(api.requests) != 2 {
			t.Fatalf("answer %+v, err %v, %d requests", ans, err, len(api.requests))
		}
	})
	t.Run("two leaks go unanswered", func(t *testing.T) {
		c, api := testClaude(t, reply{text: "crane!"})
		if _, err := c.Pray(context.Background(), major); !errors.Is(err, ErrUnanswered) || len(api.requests) != 2 {
			t.Fatalf("err %v, %d requests", err, len(api.requests))
		}
	})
}

// A real prayer to Claude. Costs about a cent, so only when asked:
//
//	NYTRPG_LIVE_PRAYER=1 go test -run TestLivePrayer -v ./internal/oracle/
func TestLivePrayer(t *testing.T) {
	if os.Getenv("NYTRPG_LIVE_PRAYER") != "1" {
		t.Skip("set NYTRPG_LIVE_PRAYER=1 (and ANTHROPIC_API_KEY) to pray for real")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := NewClaude()
	for _, p := range []Prayer{{Kind: protocol.PrayerMinor, Word: "CRANE", Pos: 2}, {Kind: protocol.PrayerMajor, Word: "CRANE"}} {
		ans, err := c.Pray(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s, %s: %s", ans.God.Name, ans.God.Title, ans.Text)
	}
}
