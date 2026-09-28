import { test } from "node:test";
import assert from "node:assert/strict";
import { cameraFor } from "../static/game-objects.js";

// A 1280x800 screen on a 4690x4690 map, like the town
const W = 1280, H = 800, MAP = 4690;
const cam = (x, y, mapW = MAP, mapH = MAP) => cameraFor({ x, y }, W, H, mapW, mapH);

test("away from the edges the camera keeps the player in the middle", () => {
    assert.deepEqual(cam(2000, 1500), { x: 2000 - W / 2, y: 1500 - H / 2 });
});

test("the camera stops at every edge of the map, so nothing past it shows", () => {
    assert.deepEqual(cam(100, 1500), { x: 0, y: 1100 }, "left");
    assert.deepEqual(cam(MAP - 100, 1500), { x: MAP - W, y: 1100 }, "right");
    assert.deepEqual(cam(2000, 100), { x: 1360, y: 0 }, "top");
    assert.deepEqual(cam(2000, MAP - 100), { x: 1360, y: MAP - H }, "bottom");
    assert.deepEqual(cam(0, 0), { x: 0, y: 0 }, "top left corner");
    assert.deepEqual(cam(MAP, MAP), { x: MAP - W, y: MAP - H }, "bottom right corner");
});

test("exactly half a screen from the edge is the last centered spot", () => {
    assert.deepEqual(cam(W / 2, H / 2), { x: 0, y: 0 });
    assert.deepEqual(cam(W / 2 - 1, H / 2 - 1), { x: 0, y: 0 });
});

test("a screen bigger than the map shows the whole map centered", () => {
    assert.deepEqual(cam(300, 200, 1000, 600), { x: -140, y: -100 });
    // Only one axis too small: the other one still clamps
    assert.deepEqual(cam(50, 200, 1000, 4000), { x: -140, y: 0 });
});

test("before the welcome (no map size) the camera just centers on the player", () => {
    assert.deepEqual(cam(100, 100, 0, 0), { x: 100 - W / 2, y: 100 - H / 2 });
});
