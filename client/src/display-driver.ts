import { GameState } from "./game-objects.js";
import { ChatMsg } from "./protocol.gen.js";
import { Vector2D } from "./vector2D.js";

// How long a chat bubble stays up
const CHAT_MS = 5000;
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

        this.loadImage("character", "Skoobyuboo.png");
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
        this.drawOtherChars();
        this.drawCharacter();
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

    private drawCharacter() {
        const sprite = this.images.get("character");
        if (sprite) {
            const m = this.middle;
            this.ctx.drawImage(sprite, m.x, m.y);
            this.drawLabels(this.state.selfName, this.state.selfId, m.x, m.y, "black");
        }
    }

    private drawOtherChars() {
        const cam = this.state.charVec;
        for (const id in this.state.otherChars) {
            const other = this.state.otherChars[id];
            const x = other.pos.x - cam.x;
            const y = other.pos.y - cam.y;
            if (!this.onScreen(x, y)) {
                continue;
            }
            const sprite = this.sprite(other.sprite);
            if (sprite) {
                this.ctx.drawImage(sprite, x, y);
                this.drawLabels(other.name, other.id, x, y, "white");
            }
        }
    }

    // Name above an entity, and its chat bubble above that
    private drawLabels(name: string, id: number, x: number, y: number, chatColor: string) {
        this.ctx.font = "26px serif";
        this.ctx.fillStyle = "black";
        this.ctx.fillText(name, x, y - 10);

        const chat = this.chats.get(id);
        if (chat && Date.now() > chat.exp) {
            this.chats.delete(id);
        } else if (chat) {
            this.ctx.fillStyle = chatColor;
            this.ctx.fillText(chat.chat.msg, x, y - 35);
            this.ctx.fillStyle = "black";
        }
    }

    private onScreen(x: number, y: number): boolean {
        return x > -CULL_MARGIN && y > -CULL_MARGIN && x < this.width + CULL_MARGIN && y < this.height + CULL_MARGIN;
    }

    public updateChat(chat: ChatMsg) {
        this.chats.set(chat.id, { chat: chat, exp: Date.now() + CHAT_MS });
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
