import { Vector2D } from "./vector2D.js"
import { ClassInfo, EntityID, Vec } from "./protocol.gen.js";
import { Animator } from "./animation.js";

// Draw other entities this far in the past, so there are always two known
// positions to move smoothly between. Two server ticks.
export const INTERP_DELAY_MS = 100;
// The server sends moves this often
const SERVER_TICK_MS = 50;

type Sample = { t: number, x: number, y: number };

// Another entity the server told us about
export class RemoteEntity {
    id: EntityID;
    name: string;
    // Players only: the character name and class id, drawn under the username
    char: string;
    cls: string;
    sprite: string;
    // Where to draw it, updated every frame by interpolate()
    pos: Vec;
    // Walking and facing, worked out from how pos changes
    anim: Animator;
    // Where it was last drawn on screen, for clicking it. null when off screen.
    hitbox: ScreenBox | null;
    // Positions from the server, oldest first
    private samples: Sample[];
    private lastInterpolated: number | null;

    constructor(id: EntityID, name: string, sprite: string, pos: Vec) {
        this.id = id;
        this.name = name;
        this.sprite = sprite;
        this.char = "";
        this.cls = "";
        this.pos = { x: pos.x, y: pos.y };
        this.samples = [{ t: performance.now(), x: pos.x, y: pos.y }];
        this.anim = new Animator();
        this.hitbox = null;
        this.lastInterpolated = null;
    }

    // A position from the server, now
    public addSample(x: number, y: number) {
        const now = performance.now();
        // Starting to move after standing still: it was at rest one tick ago,
        // not since the last sample, or it would jump instead of glide
        const last = this.samples[this.samples.length - 1];
        if (now - last.t > SERVER_TICK_MS * 1.5) {
            this.samples.push({ t: now - SERVER_TICK_MS, x: last.x, y: last.y });
        }
        this.samples.push({ t: now, x, y });
        // Keep one sample older than the render time to interpolate from
        const renderTime = now - INTERP_DELAY_MS;
        while (this.samples.length > 2 && this.samples[1].t <= renderTime) {
            this.samples.shift();
        }
    }

    // Sets pos to where the entity was INTERP_DELAY_MS ago, and animates it by
    // how far that moved it
    public interpolate(now: number) {
        const before = this.pos;
        this.pos = this.positionAt(now - INTERP_DELAY_MS);
        const dt = this.lastInterpolated === null ? 0 : (now - this.lastInterpolated) / 1000;
        this.lastInterpolated = now;
        this.anim.update(this.pos.x - before.x, this.pos.y - before.y, dt);
    }

    private positionAt(renderTime: number): Vec {
        const s = this.samples;
        let i = 0;
        while (i < s.length - 2 && s[i + 1].t <= renderTime) {
            i++;
        }
        const a = s[i];
        const b = s[Math.min(i + 1, s.length - 1)];
        if (b.t <= a.t || renderTime >= b.t) {
            return { x: b.x, y: b.y };
        }
        const f = Math.max(0, (renderTime - a.t) / (b.t - a.t));
        return { x: a.x + (b.x - a.x) * f, y: a.y + (b.y - a.y) * f };
    }
}

export type ScreenBox = { x: number, y: number, w: number, h: number };

// The entity drawn under screen point (x, y). Where sprites overlap, the one
// drawn last is on top.
export function pickEntity(entities: Iterable<RemoteEntity>, x: number, y: number): RemoteEntity | null {
    let hit: RemoteEntity | null = null;
    for (const e of entities) {
        const b = e.hitbox;
        if (b && x >= b.x && x <= b.x + b.w && y >= b.y && y <= b.y + b.h) {
            hit = e;
        }
    }
    return hit;
}

export class GameState {
    // Our position in the world
    selfPos: Vector2D;
    // Camera: the world point at the top left of the screen. Set each frame so
    // selfPos is in the middle.
    charVec: Vector2D;
    // Our own entity and name, set by the welcome message
    selfId: EntityID;
    selfName: string;
    // The character we're playing: its name and class
    selfChar: string;
    selfClass: ClassInfo | null;
    // Every class, from the welcome
    classes: ClassInfo[];
    // Our sprite and how it's animating
    selfSprite: string;
    selfAnim: Animator;
    otherChars: {[key: number]: RemoteEntity};
    clickables: {[key: string]: Clickable};

    constructor() {
        this.selfPos = new Vector2D(0, 0);
        this.charVec = new Vector2D(0, 0);
        this.selfId = 0;
        this.selfName = "";
        this.selfChar = "";
        this.selfClass = null;
        this.classes = [];
        this.selfSprite = "Skoobyuboo.png";
        this.selfAnim = new Animator();
        this.otherChars = {};
        this.clickables = {};
    }
}

export class Clickable {
    name: string;
    rect: Rect;
    action: Function;
    // How close the player must be, px from the edge. 0 = anywhere.
    range: number;
    // Sign drawn above it. Empty = none.
    label: string;

    constructor(name: string, pos: Vector2D, height: number, width: number, action: Function, range = 0, label = "") {
        this.name = name;
        this.rect = new Rect(pos, height, width);
        this.action = action;
        this.range = range;
        this.label = label;
    }

    // Same check as the server: distance from a world point to the rect's edge
    public inRange(p: Vector2D): boolean {
        if (this.range == 0) {
            return true;
        }
        const r = this.rect;
        const dx = Math.max(r.pos.x - p.x, 0, p.x - (r.pos.x + r.width));
        const dy = Math.max(r.pos.y - p.y, 0, p.y - (r.pos.y + r.height));
        return Math.hypot(dx, dy) <= this.range;
    }
}

// x1, y1 - - - |
// |            |
// | - - - x2, y2
export class Rect {
    tl: Vector2D;
    br: Vector2D;
    pos: Vector2D;
    height: number;
    width: number;

    constructor(pos: Vector2D, height: number, width: number) {
        this.pos = pos;
        this.height = height;
        this.width = width;
        this.tl = new Vector2D(pos.x, pos.y);
        this.br = new Vector2D(pos.x + width, pos.y + height);
    }

    // True if position passed is inside the rect
    public inRect(mouse: Vector2D) {
        if (mouse.x >= this.tl.x && mouse.x <= this.br.x && mouse.y >= this.tl.y && mouse.y <= this.br.y) {
            return true;
        }
        return false;
    }

    public adjust(vec: Vector2D) {
        this.tl.x = this.pos.x - vec.x;
        this.tl.y = this.pos.y - vec.y;
        this.br.x = this.tl.x + this.width;
        this.br.y = this.tl.y + this.height;
    }
}
