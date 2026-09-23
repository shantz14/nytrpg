import { InputDriver } from "./input-driver.js";

// A window over the game built from a <template>. Only one is open at a time,
// the game ignores input while it's open, and everything it registered with
// on() is cleaned up when it closes.
export class Popup {
    static current: Popup | null = null;

    root: HTMLElement;
    private input: InputDriver | null;
    private cleanups: Array<() => void>;
    private extra: HTMLElement[];
    private closed: boolean;
    onClose: (() => void) | null;

    private constructor(templateId: string, input: InputDriver | null) {
        this.root = cloneTemplate(templateId);
        this.input = input;
        this.cleanups = [];
        this.extra = [];
        this.closed = false;
        this.onClose = null;
    }

    // Opens a popup, or returns null if one is already open.
    // input is null before the game starts (login). A closable popup closes on
    // Escape and on any .exit button.
    static open(templateId: string, input: InputDriver | null, closable = true): Popup | null {
        if (Popup.current) {
            return null;
        }
        const popup = new Popup(templateId, input);
        Popup.current = popup;
        document.getElementById("container")!.appendChild(popup.root);
        input?.setPopupFocused();

        if (closable) {
            popup.on(document, "keydown", (e) => {
                if ((e as KeyboardEvent).key === "Escape") {
                    popup.close();
                }
            });
            for (const exit of popup.root.querySelectorAll(".exit")) {
                popup.on(exit, "click", () => popup.close());
            }
        }
        return popup;
    }

    // Shows another template on top of this popup, removed when this one closes
    public addLayer(templateId: string): HTMLElement {
        const layer = cloneTemplate(templateId);
        document.getElementById("container")!.appendChild(layer);
        this.extra.push(layer);
        for (const exit of layer.querySelectorAll(".exit")) {
            this.on(exit, "click", () => this.close());
        }
        return layer;
    }

    public removeLayer(layer: HTMLElement) {
        layer.remove();
        this.extra = this.extra.filter((l) => l !== layer);
    }

    // Finds an element inside the popup (or its layers)
    public q<T extends HTMLElement>(selector: string): T {
        for (const root of [this.root, ...this.extra]) {
            const el = root.matches(selector) ? root : root.querySelector(selector);
            if (el) {
                return el as T;
            }
        }
        throw new Error(`Popup has no ${selector}`);
    }

    // addEventListener that's removed when the popup closes
    public on(target: EventTarget, type: string, handler: (e: Event) => void) {
        target.addEventListener(type, handler);
        this.cleanups.push(() => target.removeEventListener(type, handler));
    }

    // Runs fn when the popup closes, e.g. to stop a timer
    public onCleanup(fn: () => void) {
        this.cleanups.push(fn);
    }

    public close() {
        if (this.closed) {
            return;
        }
        this.closed = true;
        for (const fn of this.cleanups) {
            fn();
        }
        this.root.remove();
        for (const layer of this.extra) {
            layer.remove();
        }
        if (Popup.current === this) {
            Popup.current = null;
        }
        this.input?.setGameFocused();
        this.onClose?.();
    }
}

function cloneTemplate(templateId: string): HTMLElement {
    const tpl = document.getElementById(templateId) as HTMLTemplateElement;
    const el = tpl.content.firstElementChild!.cloneNode(true) as HTMLElement;
    return el;
}
