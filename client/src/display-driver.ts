import { GameState } from "./game-objects.js";
import { ANIMATIONS, Animator, pose } from "./animation.js";
import { ChatMsg } from "./protocol.gen.js";
import { Vector2D } from "./vector2D.js";
import { classFont, classStyle, loadClassFonts } from "./class-style.js";
import { className } from "./classes.js";
import { wrapText } from "./chat-log.js";

// How long a chat bubble stays up, fading out over the last FADE_MS
const CHAT_MS = 7000;
const FADE_MS = 500;
// Matches the CSS tokens in styles.css
const UI_FONT = `"Alegreya", Georgia, serif`;
const TEXT_STRONG = "#f0eee9";
const TEXT_MUTED = "#d4d1cb";
// Dark edge around labels so they read on any background
const OUTLINE = "rgba(0, 0, 0, 0.8)";
const BUBBLE_MAX_W = 220;
const BUBBLE_LINE_H = 17;
// Entities this far off screen are still drawn, so big sprites don't pop in
const CULL_MARGIN = 200;

export class DisplayDriver {
    ctx: CanvasRenderingContext2D;
    canvas: HTMLCanvasElement;
    state: GameState;
    images: Map<string, HTMLImageElement>;
    // Keys of images still downloading
    loading: Set<string>;
    chats: Map<number, ChatData>;
    // Canvas size in CSS pixels
    width: number;
    height: number;

    constructor(ctx: CanvasRenderingContext2D, state: GameState) {
        this.ctx = ctx;
        this.canvas = ctx.canvas;
        this.state = state;
        this.images = new Map();
        this.loading = new Set();
        this.chats = new Map();
        this.width = 0;
        this.height = 0;

        this.scaleCanvas();
        window.addEventListener("resize", () => this.scaleCanvas());

        // Load the player's sprites up front so there's no blank first frame
        for (const a of Object.values(ANIMATIONS)) {
            this.sprite(a.idle);
            this.sprite(a.walk.image);
        }
        loadClassFonts();
    }

    // Screen point our own player is drawn at
    get middle(): Vector2D {
        return new Vector2D(this.width / 2, this.height / 2);
    }

    // Points the camera at us and moves click areas to match. Call before draw.
    public updateCamera() {
        const cam = this.state.charVec;
        cam.set(this.state.selfPos.x - this.width / 2, this.state.selfPos.y - this.height / 2);
        for (const name in this.state.clickables) {
            this.state.clickables[name].rect.adjust(cam);
        }
    }

    public draw() {
        this.ctx.clearRect(0, 0, this.width, this.height);
        this.drawBackground();
        this.drawClickables();
        // Labels after every sprite, so a player walking past can't cover a name
        const labels: Label[] = [];
        this.drawOtherChars(labels);
        this.drawCharacter(labels);
        for (const l of labels) {
            this.drawLabels(l);
        }
    }

    private drawBackground() {
        const bg = this.images.get("bg");
        if (bg) {
            this.ctx.drawImage(bg, -this.state.charVec.x, -this.state.charVec.y);
        }
    }

    private drawClickables() {
        for (const name in this.state.clickables) {
            const sprite = this.images.get(name);
            const pos = this.state.clickables[name].rect.tl;
            if (sprite && this.onScreen(pos.x, pos.y)) {
                this.ctx.drawImage(sprite, pos.x, pos.y);
            }
        }
    }

    private drawCharacter(labels: Label[]) {
        const m = this.middle;
        const s = this.state;
        const w = this.drawEntity(s.selfSprite, m.x, m.y, s.selfAnim);
        if (w) {
            labels.push({ id: s.selfId, name: s.selfName, char: s.selfChar, cls: s.selfClass?.id ?? "", cx: m.x + w / 2, top: m.y });
        }
    }

    private drawOtherChars(labels: Label[]) {
        const cam = this.state.charVec;
        for (const id in this.state.otherChars) {
            const other = this.state.otherChars[id];
            const x = other.pos.x - cam.x;
            const y = other.pos.y - cam.y;
            if (!this.onScreen(x, y)) {
                continue;
            }
            const w = this.drawEntity(other.sprite, x, y, other.anim);
            if (w) {
                labels.push({ id: other.id, name: other.name, char: other.char, cls: other.cls, cx: x + w / 2, top: y });
            }
        }
    }

    // Draws an entity's current pose at x, y and returns its width. 0 if its
    // image hasn't loaded yet.
    private drawEntity(sprite: string, x: number, y: number, anim: Animator): number {
        const p = pose(sprite, anim);
        const img = this.sprite(p.image);
        if (!img) {
            return 0;
        }
        if (!("sheet" in p)) {
            this.ctx.drawImage(img, x, y);
            return img.width;
        }
        const { frameWidth: w, frameHeight: h } = p.sheet;
        const sx = p.frame * w;
        if (p.mirrored) {
            // Flip around the sprite's own box so it stays in the same place
            this.ctx.save();
            this.ctx.translate(x + w, y);
            this.ctx.scale(-1, 1);
            this.ctx.drawImage(img, sx, 0, w, h, 0, 0, w, h);
            this.ctx.restore();
        } else {
            this.ctx.drawImage(img, sx, 0, w, h, x, y, w, h);
        }
        return w;
    }

    // Centered over the sprite, bottom up: the character's name and class
    // ("Merlin Wizard", the class in its own font and color), the username
    // above that, and the chat bubble on top
    private drawLabels(l: Label) {
        const ctx = this.ctx;
        ctx.save();
        ctx.textBaseline = "alphabetic";
        ctx.lineJoin = "round";
        let y = l.top - 8;

        if (l.char) {
            const nameFont = `700 14px ${UI_FONT}`;
            const clsFont = classFont(l.cls, 17);
            const clsName = className(this.state.classes, l.cls);
            ctx.font = nameFont;
            const nameW = ctx.measureText(l.char).width;
            ctx.font = clsFont;
            const clsW = ctx.measureText(clsName).width;
            const gap = 6;
            const left = l.cx - (nameW + gap + clsW) / 2;
            ctx.textAlign = "left";
            this.outlined(l.char, left, y, nameFont, TEXT_STRONG);
            this.outlined(clsName, left + nameW + gap, y, clsFont, classStyle(l.cls).color);
            y -= 17;
            ctx.textAlign = "center";
            this.outlined(l.name, l.cx, y, `500 13px ${UI_FONT}`, TEXT_MUTED);
            y -= 13;
        } else {
            ctx.textAlign = "center";
            this.outlined(l.name, l.cx, y, `700 14px ${UI_FONT}`, TEXT_STRONG);
            y -= 14;
        }

        const chat = this.chats.get(l.id);
        const left = chat ? chat.exp - Date.now() : 0;
        if (chat && left <= 0) {
            this.chats.delete(l.id);
        } else if (chat) {
            this.drawBubble(chat.chat.msg, l.cx, y - 4, Math.min(1, left / FADE_MS));
        }
        ctx.restore();
    }

    // Text with a dark edge, so it reads over grass, paths and other sprites
    private outlined(text: string, x: number, y: number, font: string, color: string) {
        const ctx = this.ctx;
        ctx.font = font;
        ctx.strokeStyle = OUTLINE;
        ctx.lineWidth = 4;
        ctx.strokeText(text, x, y);
        ctx.fillStyle = color;
        ctx.fillText(text, x, y);
    }

    // A speech bubble whose tail points down at (cx, bottom)
    private drawBubble(text: string, cx: number, bottom: number, alpha: number) {
        const ctx = this.ctx;
        ctx.font = `400 14px ${UI_FONT}`;
        const lines = wrapText(text, BUBBLE_MAX_W, (s) => ctx.measureText(s).width);
        const padX = 10, padY = 7, tail = 6, r = 6;
        const w = Math.max(...lines.map((s) => ctx.measureText(s).width)) + padX * 2;
        const h = lines.length * BUBBLE_LINE_H + padY * 2 - 4;
        const x0 = cx - w / 2, x1 = cx + w / 2;
        const y1 = bottom - tail, y0 = y1 - h;

        ctx.globalAlpha = alpha;
        ctx.beginPath();
        ctx.moveTo(x0 + r, y0);
        ctx.arcTo(x1, y0, x1, y1, r);
        ctx.arcTo(x1, y1, x0, y1, r);
        ctx.lineTo(cx + tail, y1);
        ctx.lineTo(cx, bottom);
        ctx.lineTo(cx - tail, y1);
        ctx.arcTo(x0, y1, x0, y0, r);
        ctx.arcTo(x0, y0, x1, y0, r);
        ctx.closePath();
        ctx.fillStyle = "rgba(28, 27, 26, 0.93)";
        ctx.fill();
        ctx.strokeStyle = "rgba(205, 198, 184, 0.35)";
        ctx.lineWidth = 1;
        ctx.stroke();

        ctx.fillStyle = TEXT_STRONG;
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        lines.forEach((s, i) => ctx.fillText(s, cx, y0 + padY + i * BUBBLE_LINE_H));
    }

    private onScreen(x: number, y: number): boolean {
        return x > -CULL_MARGIN && y > -CULL_MARGIN && x < this.width + CULL_MARGIN && y < this.height + CULL_MARGIN;
    }

    // A bubble over the speaker, if we can see them. Chat is global, speakers
    // out of view only show up in the chat log.
    public updateChat(chat: ChatMsg) {
        if (chat.id === this.state.selfId || chat.id in this.state.otherChars) {
            this.chats.set(chat.id, { chat: chat, exp: Date.now() + CHAT_MS });
        }
    }

    // Forget everything about an entity that left
    public removePlayer(id: number) {
        this.chats.delete(id);
    }

    // Sharp on high DPI screens: the backing store is scaled up, drawing stays
    // in CSS pixels
    private scaleCanvas() {
        const dpr = window.devicePixelRatio || 1;
        this.width = window.innerWidth;
        this.height = window.innerHeight;
        this.canvas.width = Math.round(this.width * dpr);
        this.canvas.height = Math.round(this.height * dpr);
        this.canvas.style.width = this.width + "px";
        this.canvas.style.height = this.height + "px";
        // Resizing resets the context
        this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        this.ctx.imageSmoothingEnabled = false;
    }

    // An entity sprite by file name, loaded the first time it's needed and shared
    private sprite(file: string): HTMLImageElement | undefined {
        const key = "sprite/" + file;
        const img = this.images.get(key);
        if (!img && !this.loading.has(key)) {
            this.loadImage(key, file);
        }
        return img;
    }

    public loadImage(key: string, filename: string) {
        this.loading.add(key);
        const image = new Image();
        image.src = "./assets/" + filename;
        image.onload = () => {
            this.images.set(key, image);
            this.loading.delete(key);
        };
    }

}

type ChatData = {
    chat: ChatMsg,
    exp: number
}

// A nameplate to draw once every sprite is down. cx: the sprite's center on
// screen, top: its top edge.
type Label = {
    id: number;
    name: string;
    char: string;
    cls: string;
    cx: number;
    top: number;
}
