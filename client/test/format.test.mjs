import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { formatTime } from "../static/wordle.js";
import { shiftDate } from "../static/leaderboard.js";

test("formatTime is m:ss", () => {
    assert.equal(formatTime(0), "0:00");
    assert.equal(formatTime(9.9), "0:09");
    assert.equal(formatTime(61), "1:01");
    assert.equal(formatTime(754), "12:34");   // the old version broke at 10 minutes
    assert.equal(formatTime(-5), "0:00");
});

test("shiftDate crosses months, years and leap days", () => {
    assert.equal(shiftDate("2026-09-23", 1), "2026-09-24");
    assert.equal(shiftDate("2026-09-30", 1), "2026-10-01");
    assert.equal(shiftDate("2027-01-01", -1), "2026-12-31");
    assert.equal(shiftDate("2028-03-01", -1), "2028-02-29");
});
