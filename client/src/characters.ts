import { Popup } from "./popup.js";
import { UserData, logout } from "./login.js";
import { CharacterInfo, ClassInfo } from "./protocol.gen.js";
import { className } from "./classes.js";

const charactersURL = "/characters";

type CharacterList = {
    // One per slot, null when empty
    slots: Array<CharacterInfo | null>,
    classes: ClassInfo[],
};

class ApiError extends Error {
    constructor(public status: number, message: string) {
        super(message);
    }
}

// A /characters request with the player's token. Throws with the server's
// explanation, e.g. "Name must be 1-20 characters."
async function api(user: UserData, method: string, url: string, body?: unknown): Promise<Response> {
    const headers: Record<string, string> = { Authorization: `Bearer ${user.jwt}` };
    if (body !== undefined) {
        headers["Content-Type"] = "application/json";
    }
    const res = await fetch(url, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!res.ok) {
        throw new ApiError(res.status, (await res.text()).trim() || `Status ${res.status}`);
    }
    return res;
}

// Shows the character screen until the player picks a character to play
export function selectCharacter(user: UserData): Promise<CharacterInfo> {
    return new Promise((resolve) => {
        const popup = Popup.open("tpl-characters", null, false)!;
        new CharacterScreen(popup, user, (c) => {
            popup.close();
            resolve(c);
        }).refresh();
    });
}

class CharacterScreen {
    private classes: ClassInfo[] = [];

    constructor(private popup: Popup, private user: UserData, private play: (c: CharacterInfo) => void) {
        popup.q("#charsUser").textContent = user.username;
        popup.on(popup.q("#charsLogout"), "click", logout);
    }

    private showError(error: unknown) {
        if (error instanceof ApiError && error.status == 401) {
            // The saved login expired
            logout();
            return;
        }
        console.error("Characters:", error);
        this.popup.q("#charsError").textContent = error instanceof ApiError ? error.message : "Couldn't reach the server, try again.";
    }

    public async refresh() {
        try {
            const list: CharacterList = await (await api(this.user, "GET", charactersURL)).json();
            this.classes = list.classes;
            this.popup.q("#charsError").textContent = "";
            this.render(list.slots);
        } catch (error) {
            this.showError(error);
        }
    }

    private render(slots: Array<CharacterInfo | null>) {
        const container = this.popup.q("#charSlots");
        container.replaceChildren();
        // textContent, never innerHTML: names come from users
        slots.forEach((c, slot) => {
            const card = document.createElement("div");
            card.className = "char-slot";
            card.dataset.slot = String(slot);

            if (!c) {
                card.classList.add("empty");
                const create = document.createElement("button");
                create.className = "char-create";
                create.textContent = "+ New character";
                create.addEventListener("click", () => this.openCreate(slot));
                card.appendChild(create);
                container.appendChild(card);
                return;
            }

            card.classList.add("filled", "class-" + c.class);
            const name = document.createElement("div");
            name.className = "char-name";
            name.textContent = c.name;
            const cls = document.createElement("div");
            cls.className = "char-class";
            cls.textContent = className(this.classes, c.class);

            const actions = document.createElement("div");
            actions.className = "char-actions";
            const play = document.createElement("button");
            play.className = "btn btn-primary char-play";
            play.textContent = "Play";
            play.addEventListener("click", () => this.play(c));
            const del = document.createElement("button");
            del.className = "btn btn-ghost char-delete";
            del.textContent = "Delete";
            del.addEventListener("click", () => this.confirmDelete(c));
            actions.append(play, del);

            card.append(name, cls, actions);
            container.appendChild(card);
        });
    }

    private openCreate(slot: number) {
        const layer = this.popup.addLayer("tpl-char-create");
        const q = <T extends HTMLElement>(sel: string) => layer.querySelector(sel) as T;
        const nameInput = q<HTMLInputElement>("#charName");
        const err = q<HTMLParagraphElement>("#charError");
        const submit = q<HTMLButtonElement>("#submitCreate");
        const close = () => this.popup.removeLayer(layer);

        const choices = q<HTMLDivElement>("#classChoices");
        this.classes.forEach((cls, i) => {
            const label = document.createElement("label");
            label.className = "class-choice class-" + cls.id;
            const radio = document.createElement("input");
            radio.type = "radio";
            radio.name = "charClass";
            radio.value = cls.id;
            radio.checked = i == 0;
            const name = document.createElement("span");
            name.className = "class-name";
            name.textContent = cls.name;
            const desc = document.createElement("span");
            desc.className = "class-desc";
            desc.textContent = cls.description;
            label.append(radio, name, desc);
            choices.appendChild(label);
        });

        const create = async () => {
            const chosen = layer.querySelector<HTMLInputElement>("input[name=charClass]:checked");
            const name = nameInput.value.trim();
            if (!name) {
                err.textContent = "Give your character a name.";
                return;
            }
            if (!chosen) {
                err.textContent = "Choose a class.";
                return;
            }
            submit.disabled = true;
            try {
                await api(this.user, "POST", charactersURL, { slot, name, class: chosen.value });
                close();
                await this.refresh();
            } catch (error) {
                if (error instanceof ApiError && error.status != 401) {
                    err.textContent = error.message;
                } else {
                    this.showError(error);
                }
            } finally {
                submit.disabled = false;
            }
        };
        q("#cancelCreate").addEventListener("click", close);
        submit.addEventListener("click", create);
        nameInput.addEventListener("keydown", (e) => {
            if (e.key === "Enter") {
                create();
            }
        });
        nameInput.focus();
    }

    private confirmDelete(c: CharacterInfo) {
        const layer = this.popup.addLayer("tpl-char-delete");
        const close = () => this.popup.removeLayer(layer);
        (layer.querySelector("#deleteName") as HTMLElement).textContent = c.name;
        layer.querySelector("#cancelDelete")!.addEventListener("click", close);
        const confirm = layer.querySelector("#confirmDelete") as HTMLButtonElement;
        confirm.addEventListener("click", async () => {
            confirm.disabled = true;
            try {
                await api(this.user, "DELETE", `${charactersURL}?id=${c.id}`);
            } catch (error) {
                this.showError(error);
            }
            close();
            await this.refresh();
        });
    }
}
