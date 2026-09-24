import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { ChatLog, MAX_LOG_LINES, chatLine, wrapText } from "../static/chat-log.js";

const classes = [{ id: "wizard", name: "Wizard", description: "", abilities: [] }];

test("chatLine names the speaker, and knows our own messages", () => {
    const msg = { id: 4, msg: "hi", name: "alice", char: "Merlin", class: "wizard" };
    assert.deepEqual(chatLine(msg, 9), { char: "Merlin", cls: "wizard", user: "alice", text: "hi", self: false });
    assert.equal(chatLine(msg, 4).self, true);
    // No character: omitempty fields don't arrive at all
    assert.deepEqual(chatLine({ id: 4, msg: "hi", name: "bob" }, 9), { char: "", cls: "", user: "bob", text: "hi", self: false });
});

// One unit per character, so widths are easy to reason about
const len = (s) => s.length;

test("wrapText breaks on spaces within the width", () => {
    assert.deepEqual(wrapText("the quick brown fox", 10, len), ["the quick", "brown fox"]);
    assert.deepEqual(wrapText("  spaced   out  ", 20, len), ["spaced out"]);
    assert.deepEqual(wrapText("", 10, len), []);
});

test("wrapText cuts words longer than a line", () => {
    assert.deepEqual(wrapText("aaaaaaaaaaaaaaaaaaaaaaa", 10, len), ["aaaaaaaaaa", "aaaaaaaaaa", "aaa"]);
    assert.deepEqual(wrapText("hi aaaaaaaaaaaa", 10, len), ["hi", "aaaaaaaaaa", "aa"]);
});

test("wrapText ends in an ellipsis past maxLines", () => {
    const lines = wrapText("one two three four five six seven eight", 9, len, 2);
    assert.equal(lines.length, 2);
    assert.equal(lines[0], "one two");
    assert.ok(lines[1].endsWith("…") && lines[1].length <= 9, lines[1]);
});

// Just enough DOM for ChatLog: elements with children, classes and text
function fakeDoc() {
    const make = () => {
        const el = {
            children: [], className: "", textContent: "", title: "",
            scrollHeight: 0, scrollTop: 0, clientHeight: 0,
            classList: { set: new Set(), add(c) { this.set.add(c); }, remove(c) { this.set.delete(c); } },
            get childElementCount() { return el.children.length; },
            get firstElementChild() { return el.children[0]; },
            appendChild(c) { c.parent = el; el.children.push(c); return c; },
            append(...cs) { cs.forEach((c) => el.appendChild(c)); },
            remove() { el.parent.children.splice(el.parent.children.indexOf(el), 1); },
        };
        el.ownerDocument = doc;
        return el;
    };
    const doc = { createElement: make };
    return make();
}

test("ChatLog shows who said what, safely, and keeps the last MAX_LOG_LINES", (t) => {
    t.mock.timers.enable({ apis: ["setTimeout"] });
    const el = fakeDoc();
    const log = new ChatLog(el);

    log.add({ id: 1, msg: "<b>hi</b>", name: "alice", char: "Merlin", class: "wizard" }, 1, classes);
    const [row] = el.children;
    assert.equal(row.className, "chat-line self");
    const [char, user, text] = row.children;
    assert.equal(char.className, "chat-char class-wizard");
    assert.equal(char.textContent, "Merlin");
    assert.equal(char.title, "Wizard");
    assert.equal(user.textContent, "alice");
    // As text, never parsed as HTML
    assert.equal(text.textContent, "<b>hi</b>");

    // Brightens on a new message, then fades back
    assert.ok(el.classList.set.has("active"));
    t.mock.timers.tick(6000);
    assert.ok(!el.classList.set.has("active"));

    for (let i = 0; i < MAX_LOG_LINES + 5; i++) {
        log.add({ id: 2, msg: "m" + i, name: "bob" }, 1, classes);
    }
    assert.equal(el.children.length, MAX_LOG_LINES);
    // The oldest went first
    assert.equal(el.children[0].children[1].textContent, "m5");
    // Speakers without a character are just their username
    assert.equal(el.children[0].children.length, 2);
    assert.equal(el.children[0].children[0].className, "chat-user solo");
});
