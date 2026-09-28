import { iconUrl } from "./abilities.js";
import { AbilityInfo, ClassInfo, PassiveInfo } from "./protocol.gen.js";

// The side panel describing your class's abilities. It sits over everything,
// popups included, so it can be read mid-duel.
export class AbilityPanel {
    private el: HTMLElement;
    private cls: ClassInfo | null;

    constructor(cls: ClassInfo | null) {
        this.el = document.getElementById("ability-panel")!;
        this.cls = cls;
        this.render();
        this.el.querySelector("#abilityPanelClose")!.addEventListener("click", () => this.toggle(false));
    }

    get isOpen(): boolean {
        return !this.el.hidden;
    }

    public toggle(open = !this.isOpen) {
        this.el.hidden = !open;
        document.getElementById("hudAbilities")?.setAttribute("aria-expanded", String(open));
    }

    private render() {
        const cls = this.cls;
        const title = this.el.querySelector("#abilityPanelTitle")!;
        title.textContent = cls ? `${cls.name} abilities` : "Abilities";
        title.className = "ability-panel-title" + (cls ? " class-" + cls.id : "");

        const list = this.el.querySelector("#abilityPanelList")!;
        list.replaceChildren();
        const abilities = cls?.abilities.filter((a) => a.id) ?? [];
        if (abilities.length === 0) {
            const empty = document.createElement("p");
            empty.className = "ability-panel-empty";
            empty.textContent = "Your class has no abilities yet.";
            list.appendChild(empty);
        }
        abilities.forEach((a) => list.appendChild(entry(a, cls!.abilities.indexOf(a) + 1)));
        if (cls?.passive.id) {
            list.appendChild(entry(cls.passive, null));
        }
        const energy = document.createElement("p");
        energy.className = "ability-panel-energy";
        energy.textContent = "Energy: you start a duel with none. Gain 1 every 30 seconds, 1 for each new yellow letter and 2 for each new green.";
        list.appendChild(energy);
    }
}

function entry(a: AbilityInfo | PassiveInfo, key: number | null): HTMLElement {
    const row = document.createElement("div");
    row.className = "ability-entry" + (key === null ? " passive" : "");
    row.dataset.ability = a.id;

    const icon = document.createElement("img");
    icon.className = "ability-icon";
    icon.src = iconUrl(a.icon);
    icon.alt = "";
    row.appendChild(icon);

    const text = document.createElement("div");
    text.className = "ability-entry-text";
    const head = document.createElement("div");
    head.className = "ability-entry-head";
    const name = document.createElement("span");
    name.className = "ability-entry-name";
    name.textContent = a.name;
    head.appendChild(name);
    const tag = document.createElement("span");
    tag.className = "ability-entry-tag";
    if (key === null) {
        tag.textContent = "Passive";
    } else {
        const ability = a as AbilityInfo;
        tag.textContent = `${ability.cost} energy · key ${key}` + (ability.once ? " · once" : "");
    }
    head.appendChild(tag);
    text.appendChild(head);
    const desc = document.createElement("p");
    desc.textContent = a.description;
    text.appendChild(desc);
    row.appendChild(text);
    return row;
}
