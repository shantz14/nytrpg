import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import "./setup.mjs";
import { CLASS_STYLE, classFont, classStyle } from "../static/class-style.js";

const css = readFileSync(new URL("../static/styles.css", import.meta.url), "utf8");
const goClasses = readFileSync(new URL("../../internal/classes/classes.go", import.meta.url), "utf8");

test("every server class has a style", () => {
    const ids = [...goClasses.matchAll(/^\s*\w+\s+ID = "(\w+)"/gm)].map((m) => m[1]);
    assert.ok(ids.length >= 4, `found class IDs: ${ids}`);
    for (const id of ids) {
        assert.ok(CLASS_STYLE[id], `no style for class ${id}`);
    }
});

test("unknown classes fall back to the UI font", () => {
    assert.equal(classStyle("bard").font, "IBM Plex Mono");
    assert.equal(classFont("bard", 12), `600 12px "IBM Plex Mono", serif`);
    assert.equal(classFont("wizard", 16), `400 16px "Uncial Antiqua", serif`);
});

// The canvas uses CLASS_STYLE, the DOM uses .class-<id> in styles.css
test("the CSS class colors and fonts match CLASS_STYLE", () => {
    for (const [id, s] of Object.entries(CLASS_STYLE)) {
        const rule = css.match(new RegExp(`\\.class-${id}\\s*\\{([^}]*)\\}`))?.[1];
        assert.ok(rule, `no .class-${id} rule`);
        assert.match(rule, new RegExp(`--class-color:\\s*${s.color}`, "i"), `.class-${id} color`);
        assert.match(rule, new RegExp(`--class-font:\\s*'${s.font}'`), `.class-${id} font`);
        assert.match(rule, new RegExp(`--class-weight:\\s*${s.weight}`), `.class-${id} weight`);
    }
});

test("every class font is loaded by the page", () => {
    const html = readFileSync(new URL("../static/index.html", import.meta.url), "utf8");
    for (const s of Object.values(CLASS_STYLE)) {
        assert.ok(html.includes("family=" + s.font.replaceAll(" ", "+")), `${s.font} isn't in the Google Fonts link`);
    }
});
