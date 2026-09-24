import { ClassInfo, DuelChallenge } from "./protocol.gen.js";
import { className } from "./classes.js";

type Notice = { el: HTMLElement, timer: ReturnType<typeof setTimeout> };

// Duel challenges stacked in the top right, each with Accept and Deny and a bar
// that drains until it expires. They work whatever else is open.
export class Notifications {
    private root: HTMLElement;
    private notices: Map<number, Notice>;

    constructor(root: HTMLElement) {
        this.root = root;
        this.notices = new Map();
    }

    public addChallenge(ch: DuelChallenge, classes: ClassInfo[], respond: (accept: boolean) => void) {
        this.remove(ch.id);
        const el = document.createElement("div");
        el.className = "notice";
        el.dataset.challenge = String(ch.id);

        const title = document.createElement("div");
        title.className = "notice-title";
        title.textContent = "Duel challenge";

        // textContent, never innerHTML: names come from users
        const who = document.createElement("div");
        who.className = "notice-who";
        const char = document.createElement("span");
        char.className = "notice-char";
        char.textContent = ch.char || ch.name;
        who.appendChild(char);
        if (ch.class) {
            const cls = document.createElement("span");
            cls.className = "notice-class class-" + ch.class;
            cls.textContent = className(classes, ch.class);
            who.appendChild(cls);
        }
        if (ch.char) {
            const user = document.createElement("span");
            user.className = "notice-user";
            user.textContent = ch.name;
            who.appendChild(user);
        }

        const text = document.createElement("p");
        text.className = "notice-text";
        text.textContent = "challenges you to a Wordle duel";

        const buttons = document.createElement("div");
        buttons.className = "btn-row";
        const button = (label: string, cls: string, accept: boolean) => {
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "btn " + cls;
            btn.textContent = label;
            btn.addEventListener("click", () => {
                btn.blur();
                this.remove(ch.id);
                respond(accept);
            });
            return btn;
        };
        buttons.append(button("Deny", "btn-ghost notice-deny", false), button("Accept", "btn-primary notice-accept", true));

        const bar = document.createElement("div");
        bar.className = "notice-bar";
        bar.style.animationDuration = ch.expiresMs + "ms";

        el.append(title, who, text, buttons, bar);
        this.root.appendChild(el);
        const timer = setTimeout(() => this.remove(ch.id), ch.expiresMs);
        this.notices.set(ch.id, { el, timer });
    }

    // Takes a challenge down. Returns false if it wasn't showing.
    public remove(id: number): boolean {
        const n = this.notices.get(id);
        if (!n) {
            return false;
        }
        clearTimeout(n.timer);
        n.el.remove();
        this.notices.delete(id);
        return true;
    }

    public clear() {
        for (const id of [...this.notices.keys()]) {
            this.remove(id);
        }
    }
}
