package classes

import "testing"

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
