import { AbilityInfo, ClassInfo, DuelState } from "./protocol.gen.js";

// Slots on the ability bar, matches the server's classes.AbilitySlots
export const ABILITY_SLOTS = 5;

// Ability IDs with rules beyond having the energy, from internal/classes
export const SLASH = "slash";
export const SHIELDS_UP = "shields_up";
export const CRIPPLE = "cripple";
export const ILLUSION = "illusion";
export const PICKPOCKET = "pickpocket";
export const CHEAT = "cheat";
export const MEND = "mend";
// The one ability that works from inside an Illusion
export const PURIFY = "purify";

export function iconUrl(icon: string): string {
    return "./assets/" + icon;
}

// Whether the ability can be cast now. The server has the final say, this
// just greys out the ones that would be refused.
export function castable(a: AbilityInfo, s: DuelState | null): boolean {
    if (!a.id || !s || s.you.energy < a.cost || s.you.silencedMs > 0) {
        return false;
    }
    if (s.you.illusion && a.id !== PURIFY) {
        return false;
    }
    if (a.once && s.you.used.includes(a.id)) {
        return false;
    }
    switch (a.id) {
        case SHIELDS_UP:
            return !s.you.shield;
        case CRIPPLE:
            return s.them.rows - s.them.guesses >= 2;
        case ILLUSION:
            return !s.them.illusion;
        case SLASH:
        case PICKPOCKET:
            return s.them.guesses > 0;
        case CHEAT:
            return !s.you.cheatReady;
        case MEND:
            return s.you.guesses > 0;
    }
    return true;
}

// Colors picked for Feint or Under Their Nose, as the server wants them: one
// per letter, and not all green
export function validPattern(colors: number[], wordLength: number): boolean {
    return colors.length === wordLength && colors.every((c) => c >= 0 && c <= 2) && !colors.every((c) => c === 2);
}

// The letter a key types on a keyboard scrambled by keymap ("" = not
// scrambled). Anything but A-Z is left alone.
export function remapKey(letter: string, keymap: string): string {
    const i = letter.toUpperCase().charCodeAt(0) - 65;
    return keymap && letter.length === 1 && i >= 0 && i < 26 ? keymap[i] : letter.toUpperCase();
}

// The ability buttons: icon, key and energy cost. Empty slots are disabled.
export class AbilityBar {
    private buttons: HTMLButtonElement[];
    private cls: ClassInfo | null;

    constructor(el: HTMLElement, cls: ClassInfo | null, onUse: (slot: number, ability: AbilityInfo) => void) {
        this.cls = cls;
        this.buttons = [];
        el.replaceChildren();
        for (let i = 0; i < ABILITY_SLOTS; i++) {
            const ability = cls?.abilities[i];
            const btn = document.createElement("button");
            btn.type = "button";
            btn.className = "ability-slot";
            btn.dataset.slot = String(i);

            const key = document.createElement("span");
            key.className = "ability-key";
            key.textContent = String(i + 1);
            btn.appendChild(key);

            if (ability?.id) {
                btn.classList.add("filled");
                btn.dataset.ability = ability.id;
                btn.title = `${ability.name} (${ability.cost} energy): ${ability.description}`;
                btn.setAttribute("aria-label", `${ability.name}, ${ability.cost} energy`);
                const icon = document.createElement("img");
                icon.className = "ability-icon";
                icon.src = iconUrl(ability.icon);
                icon.alt = "";
                btn.appendChild(icon);
                const cost = document.createElement("span");
                cost.className = "ability-cost";
                cost.textContent = String(ability.cost);
                btn.appendChild(cost);
                btn.addEventListener("click", () => {
                    btn.blur();
                    if (!btn.classList.contains("unaffordable")) {
                        onUse(i, ability);
                    }
                });
            } else {
                btn.classList.add("empty");
                btn.disabled = true;
                btn.title = "Empty ability slot";
            }
            el.appendChild(btn);
            this.buttons.push(btn);
        }
    }

    // Greys out what can't be cast. null greys out everything, e.g. once the
    // duel is over.
    public update(state: DuelState | null) {
        this.buttons.forEach((btn, i) => {
            const a = this.cls?.abilities[i];
            if (!a?.id) {
                return;
            }
            const ok = castable(a, state);
            btn.classList.toggle("unaffordable", !ok);
            btn.setAttribute("aria-disabled", String(!ok));
        });
    }

    // Marks the slot whose target is being picked
    public setPicking(slot: number | null) {
        this.buttons.forEach((btn, i) => btn.classList.toggle("picking", i === slot));
    }
}
