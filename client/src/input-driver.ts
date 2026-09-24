import { Vector2D } from "./vector2D.js";
import { GameState } from "./game-objects.js";

export enum InputMode {
    GameFocused = 1,
    PopupFocused,
    ChatFocused
}

export class InputDriver {
    canvas: HTMLCanvasElement;
    keysPressed: Set<string>;
    mousePos: Vector2D;
    state: GameState;
    inputMode: InputMode;
    // Called when the player clicks something they're too far away to use
    onTooFar: (() => void) | null;

    constructor(canvas: HTMLCanvasElement, state: GameState) {
        this.canvas = canvas;
        this.keysPressed = new Set();
        this.mousePos = new Vector2D(0, 0);
        this.state = state;
        this.inputMode = InputMode.GameFocused;
        this.onTooFar = null;

        const chatbox = document.getElementById("chatbox") as HTMLInputElement;

        document.addEventListener('keydown', (event) => {
            // "/" or Enter opens chat
            if ((event.key == "/" || event.key == "Enter") && this.isGameFocused()) {
                event.preventDefault();
                chatbox.focus();
                this.setChatFocused();
                return;
            }
            if (this.inputMode == InputMode.ChatFocused && (event.key == "Enter" || event.key == "Escape")) {
                // Enter sends, Escape throws the message away
                if (event.key == "Enter" && chatbox.value.trim()) {
                    chatbox.dispatchEvent(new Event("sendChat"));
                }
                chatbox.value = "";
                chatbox.blur();
                this.setGameFocused();
                return;
            }
            if (this.isGameFocused()) {
                this.keysPressed.add(event.key.toLowerCase());
            }
        });

        // Always forget released keys, even if focus moved meanwhile
        document.addEventListener('keyup', (event) => {
            this.keysPressed.delete(event.key.toLowerCase());
        });

        // Keys released while the window is in the background never send keyup
        window.addEventListener('blur', () => this.keysPressed.clear());

        chatbox.addEventListener('focus', () => {
            if (this.isGameFocused()) {
                this.setChatFocused();
            }
        });
        chatbox.addEventListener('blur', () => {
            if (this.inputMode == InputMode.ChatFocused) {
                this.setGameFocused();
            }
        });

        // Only clicks on the canvas itself reach the world, popups sit above it
        this.canvas.addEventListener('click', (event) => {
            this.updateMousePos(event);
            this.click();
        });
    }

    private click() {
        if (!this.isGameFocused()) {
            return;
        }
        for (const key in this.state.clickables) {
            const obj = this.state.clickables[key];
            if (obj.rect.inRect(this.mousePos)) {
                if (obj.inRange(this.state.selfPos)) {
                    obj.action();
                } else {
                    this.onTooFar?.();
                }
                return;
            }
        }
    }

    private updateMousePos(e: MouseEvent) {
        const rect = this.canvas.getBoundingClientRect();
        this.mousePos.set(e.clientX - rect.left, e.clientY - rect.top);
    }

    public isGameFocused(): boolean {
        return this.inputMode == InputMode.GameFocused;
    }

    public setGameFocused() {
        this.inputMode = InputMode.GameFocused;
    }

    // Stops movement: keys held when a popup opens would otherwise stay pressed
    public setPopupFocused() {
        this.inputMode = InputMode.PopupFocused;
        this.keysPressed.clear();
    }

    public setChatFocused() {
        this.inputMode = InputMode.ChatFocused;
        this.keysPressed.clear();
    }

}
