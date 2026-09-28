// Package classes is the registry of playable character classes and their
// abilities. Every character has exactly one class, stored by ID.
//
// This is scaffolding: classes have names and empty ability slots so the rest of
// the game (character select, labels, the ability bar) can be built against it.
// Stats and real abilities get added here.
package classes

import (
	"time"

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

// Something a player can activate. Not used yet, abilities will fill in the
// behavior (a func on the world or a puzzle) when the first one is made.
type Ability struct {
	ID          string
	Name        string
	Description string
	Cooldown    time.Duration
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
}

// In the order the character creation screen shows them
var all = []Class{
	{ID: Knight, Name: "Knight", Description: "A sturdy fighter in heavy armor.", Sprite: "Skoobyuboo.png"},
	{ID: Wizard, Name: "Wizard", Description: "A scholar of arcane magic.", Sprite: "wizard.png"},
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
			info.Abilities[i] = protocol.AbilityInfo{ID: a.ID, Name: a.Name, Description: a.Description}
		}
	}
	return info
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
