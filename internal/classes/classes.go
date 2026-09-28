// Package classes is the registry of playable character classes and their
// abilities. Every character has exactly one class, stored by ID.
//
// Abilities are cast in duels with energy. This package only describes them
// (name, cost, what the caster targets); what they do lives in the game
// package, keyed by ability ID.
package classes

import (
	"nytrpg/internal/protocol"
)

// Stored in the database, never rename one
type ID string

const (
	Knight ID = "knight"
	Wizard ID = "wizard"
	Rogue  ID = "rogue"
	Cleric ID = "cleric"
)

// Abilities shown on the ability bar
const AbilitySlots = 5

// Ability and passive IDs. Icons are client/static/assets/abilities/<id>.png.
const (
	Slash         = "slash"
	ShieldsUp     = "shields_up"
	Determination = "determination"
	PommelStrike  = "pommel_strike"
	Cripple       = "cripple"
	Aggressive    = "aggressive"

	Scry           = "scry"
	SeeingEye      = "seeing_eye"
	MagicMissile   = "magic_missile"
	Illusion       = "illusion"
	ReshapeReality = "reshape_reality"
	Wise           = "wise"
)

// Something a player casts in a duel for energy
type Ability struct {
	ID          string
	Name        string
	Description string
	Cost        int
	// Can only be cast once a duel
	Once bool
	// What the caster picks when casting it
	Target protocol.AbilityTarget
}

// Always on, set off by something in the duel
type Passive struct {
	ID          string
	Name        string
	Description string
}

type Class struct {
	ID          ID
	Name        string
	Description string
	// Image file in client/static/assets. The client needs a matching entry in
	// ANIMATIONS (client/src/animation.ts) for it to walk.
	Sprite string
	// nil = empty slot
	Abilities [AbilitySlots]*Ability
	// nil = none
	Passive *Passive
}

// In the order the character creation screen shows them
var all = []Class{
	{
		ID: Knight, Name: "Knight", Description: "A sturdy fighter in heavy armor.", Sprite: "Skoobyuboo.png",
		Abilities: [AbilitySlots]*Ability{
			{ID: Slash, Name: "Slash", Cost: 2, Target: protocol.TargetOpponentTile,
				Description: "Destroy a letter in one of your opponent's guesses. They can't see what it was anymore."},
			{ID: ShieldsUp, Name: "Shields Up", Cost: 2,
				Description: "Raise a shield that ignores the next enemy ability that would affect you. Only one shield at a time."},
			{ID: Determination, Name: "Determination", Cost: 3,
				Description: "Gain another guess row, even after you've run out of guesses."},
			{ID: PommelStrike, Name: "Pommel Strike", Cost: 3,
				Description: "Stun your opponent's keyboard for 10 seconds."},
			{ID: Cripple, Name: "Cripple", Cost: 8,
				Description: "Your opponent loses a guess row. Can't take their last one."},
		},
		Passive: &Passive{ID: Aggressive, Name: "Aggressive",
			Description: "Guesses that find a new green letter stun your opponent's keyboard for 5 seconds."},
	},
	{
		ID: Wizard, Name: "Wizard", Description: "A scholar of arcane magic.", Sprite: "wizard.png",
		Abilities: [AbilitySlots]*Ability{
			{ID: Scry, Name: "Scry", Cost: 2, Target: protocol.TargetLetter,
				Description: "Choose any letter and learn whether it's in your word."},
			{ID: SeeingEye, Name: "Seeing Eye", Cost: 5,
				Description: "See one random letter in your opponent's guesses, never a green one. It moves every 30 seconds. Each cast adds another eye."},
			{ID: MagicMissile, Name: "Magic Missile", Cost: 6,
				Description: "A missile flies for 15 seconds. If your opponent doesn't guess before it lands, they lose a guess row and 5 energy."},
			{ID: Illusion, Name: "Illusion", Cost: 8,
				Description: "Trap your opponent in a 3-letter Wordle they must finish before returning to the duel. If they fail, they lose 5 energy."},
			{ID: ReshapeReality, Name: "Reshape Reality", Cost: 12, Target: protocol.TargetWord,
				Description: "Pick one of 5 random words: it becomes your opponent's word, and all their guesses are recolored to match."},
		},
		Passive: &Passive{ID: Wise, Name: "Wise",
			Description: "Guesses that find a new green letter move your Seeing Eyes right away."},
	},
	{ID: Rogue, Name: "Rogue", Description: "A quick, sly trickster.", Sprite: "rogue.png"},
	{ID: Cleric, Name: "Cleric", Description: "A healer who calls on divine power.", Sprite: "cleric.png"},
}

var byID = func() map[ID]*Class {
	m := make(map[ID]*Class, len(all))
	for i := range all {
		m[all[i].ID] = &all[i]
	}
	return m
}()

func All() []Class {
	return append([]Class(nil), all...)
}

func Get(id ID) (Class, bool) {
	c, ok := byID[id]
	if !ok {
		return Class{}, false
	}
	return *c, true
}

// The ability in a class's slot, nil for an empty slot, a bad slot or an
// unknown class
func AbilityAt(id ID, slot int) *Ability {
	c, ok := byID[id]
	if !ok || slot < 0 || slot >= AbilitySlots {
		return nil
	}
	return c.Abilities[slot]
}

// The class's passive ID, empty if it has none
func PassiveOf(id ID) string {
	if c, ok := byID[id]; ok && c.Passive != nil {
		return c.Passive.ID
	}
	return ""
}

// For the client: always AbilitySlots abilities, empty ones have no ID
func (c Class) Info() protocol.ClassInfo {
	info := protocol.ClassInfo{
		ID:          string(c.ID),
		Name:        c.Name,
		Description: c.Description,
		Sprite:      c.Sprite,
		Abilities:   make([]protocol.AbilityInfo, AbilitySlots),
	}
	for i, a := range c.Abilities {
		if a != nil {
			info.Abilities[i] = protocol.AbilityInfo{
				ID: a.ID, Name: a.Name, Description: a.Description,
				Cost: a.Cost, Once: a.Once, Target: a.Target, Icon: Icon(a.ID),
			}
		}
	}
	if p := c.Passive; p != nil {
		info.Passive = protocol.PassiveInfo{ID: p.ID, Name: p.Name, Description: p.Description, Icon: Icon(p.ID)}
	}
	return info
}

// An ability's or passive's icon, in client/static/assets
func Icon(id string) string {
	return "abilities/" + id + ".png"
}

// The sprite a player of this class is drawn with. Unknown classes look like
// a knight.
func Sprite(id ID) string {
	if c, ok := byID[id]; ok && c.Sprite != "" {
		return c.Sprite
	}
	return byID[Knight].Sprite
}

// Every class, for the client
func Infos() []protocol.ClassInfo {
	infos := make([]protocol.ClassInfo, len(all))
	for i, c := range all {
		infos[i] = c.Info()
	}
	return infos
}
