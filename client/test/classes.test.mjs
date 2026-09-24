import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { charLabel, className } from "../static/classes.js";

const classes = [
    { id: "knight", name: "Knight", description: "", abilities: [] },
    { id: "wizard", name: "Wizard", description: "", abilities: [] },
];

test("className looks up the display name", () => {
    assert.equal(className(classes, "wizard"), "Wizard");
    // A class this client doesn't know yet still shows something
    assert.equal(className(classes, "bard"), "bard");
    assert.equal(className([], "knight"), "knight");
});

test("charLabel is 'Name (Class)', empty for non-characters", () => {
    assert.equal(charLabel(classes, "Merlin", "wizard"), "Merlin (Wizard)");
    assert.equal(charLabel(classes, "Arthur", "knight"), "Arthur (Knight)");
    assert.equal(charLabel(classes, "", "knight"), "");
});
