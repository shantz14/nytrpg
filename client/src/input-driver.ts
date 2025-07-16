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

    constructor(canvas: HTMLCanvasElement, state: GameState) {
        this.canvas = canvas;
        this.keysPressed = new Set();
        this.mousePos = new Vector2D(0, 0);
        this.state = state;
        this.inputMode = InputMode.GameFocused;

        document.addEventListener('keydown', (event) => {
            if (event.key == "/") {
                event.preventDefault();
                const chatbox = document.getElementById("chatbox") as HTMLInputElement;
                chatbox.focus();
                this.setChatFocused();
            } else if (event.key == "Enter" ){
                const chatbox = document.getElementById("chatbox") as HTMLInputElement;
                if (chatbox.value) {
                    const event = new Event("sendChat");
                    chatbox.dispatchEvent(event);
                    chatbox.value = "";
                    chatbox.blur();
                    this.setGameFocused();
                }
            }
            if (this.inputMode == InputMode.GameFocused) {
                const key = event.key.toLowerCase();
                this.keysPressed.add(key);
            }
        });

        document.addEventListener('keyup', (event) => {
            if (this.inputMode == InputMode.GameFocused) {
                const key = event.key.toLowerCase();
                this.keysPressed.delete(key);
            }
        });

        document.addEventListener('mousemove', (event) => {
            this.updateMousePos(event);
        });

        document.addEventListener('mouseup', (event) => {
            this.click(event);
        });
    }

    private click(e: MouseEvent) {
        for (const key in this.state.clickables) {
            const obj = this.state.clickables[key];
            if (obj.rect.inRect(this.mousePos) && (this.inputMode = InputMode.GameFocused)) {
                obj.action();
            }
        }
    }

    private updateMousePos(e: MouseEvent) {
        let rect = this.canvas.getBoundingClientRect();

        let x = e.clientX - rect.left;
        let y = e.clientY - rect.top;

        this.mousePos.set(x, y);
    }

    public isGameFocused(): boolean {
        return this.inputMode == InputMode.GameFocused;
    }

    public setGameFocused() {
        this.inputMode = InputMode.GameFocused;
    }

    public setPopupFocused() {
        this.inputMode = InputMode.PopupFocused;
    }

    public setChatFocused() {
        this.inputMode = InputMode.ChatFocused;
    }

}
