import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { castable } from "../static/abilities.js";

const side = (over = {}) => ({
    energy: 0, rows: 5, guesses: 0, stunnedMs: 0, shield: false, eyes: 0, missilesMs: [], illusion: false, used: [], ...over,
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
