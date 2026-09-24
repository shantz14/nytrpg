import { ClassInfo, EntityID, Vec } from "./protocol.gen.js";
import { RemoteEntity } from "./game-objects.js";
import { className } from "./classes.js";
import { Vector2D } from "./vector2D.js";

// A small card that opens where you clicked another player: who they are, and
// a button to challenge them to a duel. It stays on that spot in the world when
// they walk off, and doesn't stop you moving.
export class PlayerCard {
    private el: HTMLElement;
    private duelBtn: HTMLButtonElement;
    // World point the card points at
    private anchor: Vec | null;
    private target: EntityID;
    private onDuel: ((target: EntityID) => void) | null;

    constructor(el: HTMLElement) {
        this.el = el;
        this.duelBtn = el.querySelector("#pcDuel")!;
        this.anchor = null;
        this.target = 0;
        this.onDuel = null;
        this.duelBtn.addEventListener("click", () => {
            // Keyboard focus would let Space or Enter click it again
            this.duelBtn.blur();
            if (this.anchor) {
                this.onDuel?.(this.target);
            }
            this.hide();
        });
        document.addEventListener("keydown", (e) => {
            if (e.key === "Escape") {
                this.hide();
            }
        });
    }

    get open(): boolean {
        return this.anchor !== null;
    }

    // Opens over the player where they're standing now. canDuel false greys
    // out the button, e.g. while you're in a duel.
    public show(e: RemoteEntity, classes: ClassInfo[], canDuel: boolean, onDuel: (target: EntityID) => void) {
        const w = e.hitbox?.w ?? 0;
        this.anchor = { x: e.pos.x + w / 2, y: e.pos.y };
        this.target = e.id;
        this.onDuel = onDuel;

        const char = this.el.querySelector<HTMLElement>(".pc-char")!;
        const cls = this.el.querySelector<HTMLElement>(".pc-class")!;
        const user = this.el.querySelector<HTMLElement>(".pc-user")!;
        // textContent, never innerHTML: names come from users
        char.textContent = e.char || e.name;
        cls.textContent = e.cls ? className(classes, e.cls) : "";
        cls.className = "pc-class" + (e.cls ? " class-" + e.cls : "");
        user.textContent = e.char ? e.name : "";
        this.duelBtn.disabled = !canDuel;

        this.el.hidden = false;
        // Replay the entrance animation each time
        this.el.classList.remove("pc-enter");
        void this.el.offsetWidth;
        this.el.classList.add("pc-enter");
    }

    public hide() {
        this.anchor = null;
        this.el.hidden = true;
    }

    // Keeps the card on its spot as the camera moves. Call every frame.
    public position(cam: Vector2D) {
        if (!this.anchor) {
            return;
        }
        this.el.style.left = Math.round(this.anchor.x - cam.x) + "px";
        this.el.style.top = Math.round(this.anchor.y - cam.y) + "px";
    }
}
