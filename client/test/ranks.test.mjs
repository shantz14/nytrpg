import { test } from "node:test";
import assert from "node:assert/strict";
import { tierFor, nextTier, tierProgress, signed, RANK_COLORS } from "../static/ranks.js";

// The ladder the server sends (internal/ranked/ladder.go)
const LADDER = (() => {
    const tiers = [];
    for (const family of ["iron", "bronze", "silver", "gold", "diamond"]) {
        for (const div of [3, 2, 1]) {
            const name = family[0].toUpperCase() + family.slice(1) + " " + div;
            tiers.push({ id: `${family}-${div}`, family, name, minElo: 400 + 100 * tiers.length });
        }
    }
    tiers[0].minElo = 0;
    tiers.push({ id: "master", family: "master", name: "Master", minElo: 1900 });
    return tiers;
})();

test("tierFor names the rank an elo is in", () => {
    const at = (elo) => tierFor(elo, LADDER).name;
    assert.equal(at(0), "Iron 3");
    assert.equal(at(499), "Iron 3");
    assert.equal(at(500), "Iron 2");
    assert.equal(at(1000), "Silver 3");
    assert.equal(at(1099), "Silver 3");
    assert.equal(at(1100), "Silver 2");
    assert.equal(at(1899), "Diamond 1");
    assert.equal(at(1900), "Master");
    assert.equal(at(3000), "Master");
    assert.equal(tierFor(1000, []), null, "no ladder yet, before the welcome");
});

test("nextTier and tierProgress show how far to the next rank", () => {
    assert.equal(nextTier(1050, LADDER).name, "Silver 2");
    assert.equal(nextTier(1950, LADDER), null, "nothing above Master");
    assert.equal(tierProgress(1050, LADDER), 0.5);
    assert.equal(tierProgress(1000, LADDER), 0);
    assert.equal(tierProgress(450, LADDER), 0.5, "Iron 3 counts as 100 wide like the rest");
    assert.equal(tierProgress(10, LADDER), 0);
    assert.equal(tierProgress(2500, LADDER), 1);
});

test("every rank family has a color", () => {
    for (const t of LADDER) {
        assert.match(RANK_COLORS[t.family], /^#[0-9a-f]{6}$/, t.family);
    }
});

test("signed shows the sign, with a real minus", () => {
    assert.equal(signed(12), "+12");
    assert.equal(signed(-14), "−14");
    assert.equal(signed(0), "0");
});
