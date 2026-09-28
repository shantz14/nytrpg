package classes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	want := []ID{Knight, Wizard, Rogue, Cleric}
	all := All()
	if len(all) != len(want) {
		t.Fatalf("want %d classes, got %d", len(want), len(all))
	}
	for i, id := range want {
		if all[i].ID != id || all[i].Name == "" {
			t.Fatalf("class %d: want %s, got %+v", i, id, all[i])
		}
		c, ok := Get(id)
		if !ok || c.ID != id {
			t.Fatalf("Get(%s) = %+v %v", id, c, ok)
		}
	}
	if _, ok := Get("bard"); ok {
		t.Fatal("unknown class found")
	}
	// All() is a copy, callers can't change the registry
	all[0].Name = "changed"
	if c, _ := Get(Knight); c.Name != "Knight" {
		t.Fatal("All() exposed the registry")
	}
}

func TestInfoHasEveryAbilitySlot(t *testing.T) {
	c := Class{ID: "test", Name: "Test"}
	c.Abilities[2] = &Ability{ID: "zap", Name: "Zap"}
	info := c.Info()
	if len(info.Abilities) != AbilitySlots {
		t.Fatalf("want %d slots, got %d", AbilitySlots, len(info.Abilities))
	}
	if info.Abilities[2].ID != "zap" || info.Abilities[0].ID != "" || info.Abilities[4].ID != "" {
		t.Fatalf("filled slot or empty slots wrong: %+v", info.Abilities)
	}
	if len(Infos()) != len(All()) {
		t.Fatal("Infos should cover every class")
	}
}

func TestEveryClassHasItsOwnSprite(t *testing.T) {
	seen := map[string]ID{}
	for _, c := range All() {
		if c.Sprite == "" {
			t.Fatalf("%s has no sprite", c.ID)
		}
		if other, ok := seen[c.Sprite]; ok {
			t.Fatalf("%s and %s share sprite %s", c.ID, other, c.Sprite)
		}
		seen[c.Sprite] = c.ID
		if Sprite(c.ID) != c.Sprite || c.Info().Sprite != c.Sprite {
			t.Fatalf("%s: Sprite() or Info() lost the sprite", c.ID)
		}
	}
	if Sprite("bard") != Sprite(Knight) {
		t.Fatal("unknown classes should look like a knight")
	}
}

func TestEveryClassHasItsAbilities(t *testing.T) {
	want := map[ID]struct {
		abilities []string
		costs     []int
		passive   string
	}{
		Knight: {[]string{Slash, ShieldsUp, Determination, PommelStrike, Cripple}, []int{2, 2, 3, 3, 8}, Aggressive},
		Wizard: {[]string{Scry, SeeingEye, MagicMissile, Illusion, ReshapeReality}, []int{2, 5, 6, 8, 12}, Wise},
		Rogue:  {[]string{Pickpocket, Cheat, Feint, UnderTheirNose, Confuse}, []int{2, 5, 6, 7, 10}, Sneaky},
		Cleric: {[]string{MinorPrayer, Mend, Purify, MajorPrayer, DivineIntervention}, []int{2, 3, 4, 6, 10}, DivineWill},
	}
	if len(want) != len(All()) {
		t.Fatal("a class is missing from the test")
	}
	for id, w := range want {
		info := byID[id].Info()
		for slot, a := range info.Abilities {
			if a.ID != w.abilities[slot] || a.Cost != w.costs[slot] || a.Name == "" || a.Description == "" || a.Once {
				t.Fatalf("%s slot %d: %+v", id, slot, a)
			}
			if AbilityAt(id, slot).ID != a.ID {
				t.Fatalf("AbilityAt(%s, %d)", id, slot)
			}
		}
		if info.Passive.ID != w.passive || PassiveOf(id) != w.passive || info.Passive.Description == "" {
			t.Fatalf("%s passive %+v", id, info.Passive)
		}
	}
	if AbilityAt(Knight, 5) != nil || AbilityAt(Knight, -1) != nil || AbilityAt("bard", 0) != nil {
		t.Fatal("AbilityAt outside the slots")
	}
	if PassiveOf("bard") != "" {
		t.Fatal("unknown classes have no passive")
	}
}

// Every ability and passive needs its pixel art (make icons)
func TestEveryAbilityHasAnIcon(t *testing.T) {
	for _, c := range Infos() {
		icons := []string{c.Passive.Icon}
		for _, a := range c.Abilities {
			icons = append(icons, a.Icon)
		}
		for _, icon := range icons {
			if icon == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join("../../client/static/assets", icon)); err != nil {
				t.Errorf("%s: %v", c.ID, err)
			}
		}
	}
}
