import { AbilityInfo, ClassInfo } from "./protocol.gen.js";

// Slots on the ability bar, matches the server's classes.AbilitySlots
export const ABILITY_SLOTS = 5;

// Fills el with one button per ability slot. Empty slots are disabled.
// Nothing activates yet: when abilities exist, onUse gets the slot clicked.
export function renderAbilityBar(el: HTMLElement, cls: ClassInfo | null, onUse?: (slot: number, ability: AbilityInfo) => void) {
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
            btn.title = `${ability.name}: ${ability.description}`;
            const name = document.createElement("span");
            name.className = "ability-name";
            name.textContent = ability.name;
            btn.appendChild(name);
            if (onUse) {
                btn.addEventListener("click", () => onUse(i, ability));
            }
        } else {
            btn.classList.add("empty");
            btn.disabled = true;
            btn.title = "Empty ability slot";
        }
        el.appendChild(btn);
    }
}
