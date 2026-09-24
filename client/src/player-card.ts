import { ClassInfo, EntityID, RankTier, Vec } from "./protocol.gen.js";
import { RemoteEntity } from "./game-objects.js";
import { className } from "./classes.js";
import { tierFor } from "./ranks.js";
import { Vector2D } from "./vector2D.js";

export type CardActions = {
    // Challenge them, ranked or not
    duel: (target: EntityID, ranked: boolean) => void;
    // Open their profile
    info: (target: EntityID) => void;
};

// A small card that opens where you clicked another player: who they are,
// their rank, and buttons to duel them or see their profile. It stays on that
// spot in the world when they walk off, and doesn't stop you moving.
export class PlayerCard {
    private el: HTMLElement;
    private duelBtn: HTMLButtonElement;
    private rankedBtn: HTMLButtonElement;
    // World point the card points at
    private anchor: Vec | null;
    private target: EntityID;
    private actions: CardActions | null;

    constructor(el: HTMLElement) {
        this.el = el;
        this.duelBtn = el.querySelector("#pcDuel")!;
        this.rankedBtn = el.querySelector("#pcRanked")!;
        this.anchor = null;
        this.target = 0;
        this.actions = null;
        const button = (btn: HTMLButtonElement, action: (a: CardActions, target: EntityID) => void) => {
            btn.addEventListener("click", () => {
                // Keyboard focus would let Space or Enter click it again
                btn.blur();
                if (this.anchor && this.actions) {
                    action(this.actions, this.target);
                }
                this.hide();
            });
        };
        button(this.duelBtn, (a, t) => a.duel(t, false));
        button(this.rankedBtn, (a, t) => a.duel(t, true));
        button(el.querySelector("#pcInfo")!, (a, t) => a.info(t));
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
    // out the duel buttons, e.g. while you're in a duel.
    public show(e: RemoteEntity, classes: ClassInfo[], ladder: RankTier[], canDuel: boolean, actions: CardActions) {
        const w = e.hitbox?.w ?? 0;
        this.anchor = { x: e.pos.x + w / 2, y: e.pos.y };
        this.target = e.id;
        this.actions = actions;

        const q = (sel: string) => this.el.querySelector<HTMLElement>(sel)!;
        // textContent, never innerHTML: names come from users
        q(".pc-char").textContent = e.char || e.name;
        const cls = q(".pc-class");
        cls.textContent = e.cls ? className(classes, e.cls) : "";
        cls.className = "pc-class" + (e.cls ? " class-" + e.cls : "");
        q(".pc-user").textContent = e.char ? e.name : "";
        const tier = tierFor(e.elo, ladder);
        const rank = q(".pc-rank");
        rank.textContent = tier ? `${tier.name} · ${e.elo}` : "";
        rank.className = "pc-rank" + (tier ? " rank-" + tier.family : "");
        this.duelBtn.disabled = !canDuel;
        this.rankedBtn.disabled = !canDuel;

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
