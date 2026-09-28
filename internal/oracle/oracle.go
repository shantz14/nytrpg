// Package oracle is the gods a Cleric prays to in duels. Each prayer is
// answered by one of them, in their own voice, with a riddle about the
// Cleric's word. Claude (claude.go) speaks for them.
package oracle

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"nytrpg/internal/protocol"
)

// Answers prayers. Pray may be slow (a network call), so it's never called on
// the world goroutine.
type Oracle interface {
	Pray(ctx context.Context, p Prayer) (Answer, error)
}

type Prayer struct {
	Kind protocol.PrayerKind
	// The Cleric's word, upper case
	Word string
	// Minor prayers: which letter of the word to hint at, from 0
	Pos int
}

type Answer struct {
	God  God
	Text string
}

// Nobody answered: the call failed, Claude declined, or every answer gave
// the word away
var ErrUnanswered = errors.New("the gods are silent")

type God struct {
	Name  string
	Title string
	// CSS color for their name in the prayer window
	Color string
	// How they speak
	Persona string
}

// Whoever hears a prayer is chosen at random
var Gods = []God{
	{"Vaelith", "Keeper of Riddles", "#9d8cf2",
		"You are ancient and patient. You speak in archaic, formal verse, like an oracle carved in stone."},
	{"Brakka", "the Thunder Jester", "#f2a93b",
		"You are loud, boastful and love a terrible pun. You speak in booming bursts and laugh at your own jokes, but your hint is always real."},
	{"Oma Sel", "Mother of Tides", "#5fb4d9",
		"You are gentle and warm, and everything reminds you of the sea. You speak softly, in flowing images of water, shells and moonlight."},
	{"Nix", "the Whispering Void", "#a3a8b3",
		"You are cold and terse. Few words, all lowercase, unsettling, as if whispered from the dark between the stars."},
	{"Sol Aureus", "the Radiant", "#e8c872",
		"You are grandiose and vain. You speak of yourself in the third person and of light, gold and glory."},
}

// What the god is and the rules every answer keeps
func systemPrompt(g God, wordLength int) string {
	return fmt.Sprintf(`You are %s, %s, a god in a fantasy word game. A mortal cleric, in the middle of a duel, prays to you for help guessing their secret %d-letter word.

%s

How you answer:
- Reply only with your words to the mortal: no preamble, quotation marks, stage directions or formatting.
- At most 60 words.
- Answer with a vague, mysterious riddle, but make it accurate, so a clever mortal can work it out.
- Never state the answer outright.`, g.Name, g.Title, wordLength, g.Persona)
}

// What the mortal prays for. Minor prayers only name the one letter, so the
// rest of the word can't slip out.
func userPrompt(p Prayer) string {
	if p.Kind == protocol.PrayerMinor {
		return fmt.Sprintf(`The mortal prays for a hint about one letter of their word. The letter is %c, and it is the %s letter of the word.

Give a riddle that hints at both the letter and its position. Don't write the letter on its own or spell it out.`,
			p.Word[p.Pos], ordinal(p.Pos+1))
	}
	return fmt.Sprintf(`The mortal prays for a hint about their whole word. The word is %s.

Give a cryptic riddle about the word. Never write the word, or any form of it.`, p.Word)
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	}
	return fmt.Sprintf("%dth", n)
}

// Whether an answer gives the word away. Minor prayers aren't checked, their
// letter is in all kinds of words.
func leaks(p Prayer, text string) bool {
	return p.Kind == protocol.PrayerMajor && strings.Contains(strings.ToUpper(text), p.Word)
}
