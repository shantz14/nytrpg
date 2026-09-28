import { DuelPrayer, PrayerMinor } from "./protocol.gen.js";

// How fast a god's words appear
const CHARS_PER_TICK = 2;
const TICK_MS = 30;
// How long the gods' proclamation stays up
const BANNER_MS = 3400;

// The Cleric's prayer window, docked on the left: each prayer, who answered
// it, and what they said. It sits over the duel, and stays until closed.
export class PrayerWindow {
    private el: HTMLElement;
    private list: HTMLElement;
    private timers: Set<number>;

    constructor() {
        this.el = document.getElementById("prayer-window")!;
        this.list = this.el.querySelector("#prayerList")!;
        this.timers = new Set();
        this.el.querySelector("#prayerClose")!.addEventListener("click", () => this.el.hidden = true);
    }

    // A fresh window for a new duel
    public open() {
        for (const t of this.timers) {
            clearInterval(t);
        }
        this.timers.clear();
        this.list.replaceChildren();
        const empty = document.createElement("p");
        empty.className = "prayer-empty";
        empty.textContent = "Pray, and the gods may answer here.";
        this.list.appendChild(empty);
        this.el.hidden = false;
    }

    public handle(p: DuelPrayer) {
        this.el.hidden = false;
        this.list.querySelector(".prayer-empty")?.remove();
        let entry = this.list.querySelector<HTMLElement>(`[data-prayer="${p.id}"]`);
        if (!entry) {
            entry = document.createElement("article");
            entry.className = "prayer pending";
            entry.dataset.prayer = String(p.id);
            const kind = document.createElement("p");
            kind.className = "prayer-kind";
            kind.textContent = p.kind === PrayerMinor ? "Minor prayer" : "Major prayer";
            const body = document.createElement("p");
            body.className = "prayer-text";
            body.textContent = "Praying to the gods…";
            entry.append(kind, body);
            this.list.prepend(entry);
        }
        if (p.pending) {
            return;
        }
        entry.classList.remove("pending");
        const body = entry.querySelector<HTMLElement>(".prayer-text")!;
        if (p.failed) {
            entry.classList.add("failed");
            body.textContent = `The gods are silent. ${p.refunded} energy returned.`;
            return;
        }
        const god = document.createElement("header");
        god.className = "prayer-god";
        god.style.setProperty("--god-color", p.godColor);
        const name = document.createElement("span");
        name.className = "prayer-god-name";
        name.textContent = p.god;
        const title = document.createElement("span");
        title.className = "prayer-god-title";
        title.textContent = p.godTitle;
        god.append(name, title);
        entry.insertBefore(god, body);
        entry.classList.add("answered");
        this.reveal(body, p.text);
    }

    // The words appear a few at a time, as if spoken
    private reveal(el: HTMLElement, text: string) {
        el.textContent = "";
        el.dataset.full = text;
        let shown = 0;
        const t = setInterval(() => {
            shown += CHARS_PER_TICK;
            el.textContent = text.slice(0, shown);
            if (shown >= text.length) {
                clearInterval(t);
                this.timers.delete(t);
            }
        }, TICK_MS);
        this.timers.add(t);
    }
}

// Divine Intervention: the gods' words across the whole screen
export function proclaim(text: string) {
    const el = document.getElementById("divine-banner")!;
    const words = el.querySelector("#divineText")!;
    words.textContent = text;
    el.hidden = false;
    el.classList.remove("show");
    void el.offsetWidth;
    el.classList.add("show");
    clearTimeout(bannerTimer);
    bannerTimer = setTimeout(() => el.hidden = true, BANNER_MS);
}
let bannerTimer: number | undefined;
