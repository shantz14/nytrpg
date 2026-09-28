import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { castable, remapKey, validPattern } from "../static/abilities.js";

const side = (over = {}) => ({
    energy: 0, rows: 5, guesses: 0, stunnedMs: 0, shield: false, eyes: 0, missilesMs: [], illusion: false, used: [],
    silencedMs: 0, scrambledGuesses: 0, keymap: "", cheatReady: false, feintReady: false, ...over,
});
const state = (you = {}, them = {}) => ({ you: side(you), them: side(them) });
const ability = (id, cost, over = {}) => ({ id, name: id, description: "", cost, once: false, target: 0, icon: "", ...over });

test("castable: needs the energy, a filled slot and a duel", () => {
    const pommel = ability("pommel_strike", 3);
    assert.equal(castable(pommel, state({ energy: 2 })), false);
    assert.equal(castable(pommel, state({ energy: 3 })), true);
    assert.equal(castable(pommel, null), false, "no duel state yet, or the duel ended");
    assert.equal(castable(ability("", 0), state({ energy: 9 })), false, "empty slot");
});

test("castable: one shield at a time", () => {
    const shield = ability("shields_up", 2);
    assert.equal(castable(shield, state({ energy: 5 })), true);
    assert.equal(castable(shield, state({ energy: 5, shield: true })), false);
});

test("castable: cripple never takes the opponent's last row", () => {
    const cripple = ability("cripple", 8);
    assert.equal(castable(cripple, state({ energy: 8 }, { rows: 5, guesses: 3 })), true);
    assert.equal(castable(cripple, state({ energy: 8 }, { rows: 5, guesses: 4 })), false);
});

test("castable: slash needs a guess to hit, illusion one at a time", () => {
    const slash = ability("slash", 2, { target: 1 });
    assert.equal(castable(slash, state({ energy: 2 }, { guesses: 0 })), false);
    assert.equal(castable(slash, state({ energy: 2 }, { guesses: 1 })), true);
    const illusion = ability("illusion", 8);
    assert.equal(castable(illusion, state({ energy: 8 })), true);
    assert.equal(castable(illusion, state({ energy: 8 }, { illusion: true })), false);
});

test("castable: once-only abilities, once", () => {
    const once = ability("big_one", 1, { once: true });
    assert.equal(castable(once, state({ energy: 5 })), true);
    assert.equal(castable(once, state({ energy: 5, used: ["big_one"] })), false);
});

test("castable: nothing while silenced, only purify inside an illusion", () => {
    const pommel = ability("pommel_strike", 3);
    assert.equal(castable(pommel, state({ energy: 9, silencedMs: 4000 })), false);
    assert.equal(castable(ability("purify", 4), state({ energy: 9, silencedMs: 4000 })), false, "not even purify");
    assert.equal(castable(pommel, state({ energy: 9, illusion: true })), false);
    assert.equal(castable(ability("purify", 4), state({ energy: 9, illusion: true })), true);
});

test("castable: cheat once at a time, mend and pickpocket need a guess", () => {
    const cheat = ability("cheat", 5);
    assert.equal(castable(cheat, state({ energy: 5 })), true);
    assert.equal(castable(cheat, state({ energy: 5, cheatReady: true })), false);
    const mend = ability("mend", 3, { target: 4 });
    assert.equal(castable(mend, state({ energy: 3, guesses: 0 })), false);
    assert.equal(castable(mend, state({ energy: 3, guesses: 2 })), true);
    const pickpocket = ability("pickpocket", 2, { target: 1 });
    assert.equal(castable(pickpocket, state({ energy: 2 }, { guesses: 0 })), false);
    assert.equal(castable(pickpocket, state({ energy: 2 }, { guesses: 1 })), true);
});

test("validPattern: a color per letter, and never all green", () => {
    assert.equal(validPattern([2, 1, 0, 0, 2], 5), true);
    assert.equal(validPattern([2, 2, 2, 2, 2], 5), false);
    assert.equal(validPattern([2, 1, 0], 5), false);
    assert.equal(validPattern([2, 1, 0, 0, 3], 5), false, "Hidden isn't a color you can pick");
});

test("remapKey: a scrambled keyboard types other letters", () => {
    const swapped = "BACDEFGHIJKLMNOPQRSTUVWXYZ";
    assert.equal(remapKey("a", swapped), "B");
    assert.equal(remapKey("B", swapped), "A");
    assert.equal(remapKey("z", swapped), "Z");
    assert.equal(remapKey("q", ""), "Q", "not scrambled");
});
