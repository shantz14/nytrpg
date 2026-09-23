import { test } from "node:test";
import assert from "node:assert/strict";
import "./setup.mjs";
import { Animator, ANIMATIONS, STOP_GRACE_S, pose } from "../static/animation.js";

const PLAYER = "Skoobyuboo.png";
const sheet = ANIMATIONS[PLAYER].walk;
const frame = 1 / sheet.fps;

// Walks dx per step for n steps of dt seconds
function walk(anim, dx, dy, n, dt = frame) {
    for (let i = 0; i < n; i++) anim.update(dx, dy, dt);
}

test("the player sheet is 5 frames of 64x64", () => {
    assert.equal(sheet.image, "player-walk.png");
    assert.equal(sheet.frames, 5);
    assert.equal(sheet.frameWidth, 64);
    assert.equal(sheet.frameHeight, 64);
});

test("frames cycle through the sheet while walking and wrap around", () => {
    const anim = new Animator();
    const seen = [];
    for (let i = 0; i < sheet.frames * 2; i++) {
        anim.update(5, 0, i === 0 ? 0 : frame);
        seen.push(anim.frame(sheet));
    }
    assert.deepEqual(seen, [0, 1, 2, 3, 4, 0, 1, 2, 3, 4]);
});

test("faces the way it last moved horizontally", () => {
    const anim = new Animator();
    walk(anim, -5, 0, 1);
    assert.equal(anim.facing, -1);
    walk(anim, 0, 5, 3); // straight down: keeps facing left
    assert.equal(anim.facing, -1);
    walk(anim, 5, 5, 1); // diagonal right
    assert.equal(anim.facing, 1);
});

test("the sheet faces right, so walking left is drawn mirrored", () => {
    const anim = new Animator();
    walk(anim, 5, 0, 1);
    assert.equal(pose(PLAYER, anim).mirrored, false);
    walk(anim, -5, 0, 1);
    assert.equal(pose(PLAYER, anim).mirrored, true);
});

test("keeps walking briefly after stopping, then faces the camera", () => {
    const anim = new Animator();
    assert.deepEqual(pose(PLAYER, anim), { image: "Skoobyuboo.png" }, "idle before ever moving");

    walk(anim, 5, 0, 3);
    anim.update(0, 0, STOP_GRACE_S / 2);
    assert.ok(anim.walking, "a short gap between moves isn't a stop");
    assert.equal(pose(PLAYER, anim).image, "player-walk.png");

    anim.update(0, 0, STOP_GRACE_S);
    assert.ok(!anim.walking);
    assert.deepEqual(pose(PLAYER, anim), { image: "Skoobyuboo.png" });

    // Walking again starts from the first frame
    anim.update(5, 0, frame);
    assert.equal(anim.frame(sheet), 0);
});

test("sprites without animations are drawn as plain images", () => {
    const anim = new Animator();
    walk(anim, 5, 0, 3);
    assert.deepEqual(pose("GameBoard.png", anim), { image: "GameBoard.png" });
});
