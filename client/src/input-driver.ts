import { GameState } from "./game-objects.js";

export class InputDriver {
    state: GameState;

    constructor(state: GameState) {
        this.state = state;

        let key: string;
        let me = this;
        document.addEventListener('keydown', function(event) {
            key = event.key;
            me.handleKeyboardInput(key);
        });
    }

    private handleKeyboardInput(key: string) {
        if (key.toLowerCase() == "w") {
            this.state.charVec.y -= 15;
        }
        else if (key.toLowerCase() == "s") {
            this.state.charVec.y += 15;
        }
        else if (key.toLowerCase() == "a") {
            this.state.charVec.x -= 15;
        }
        else if (key.toLowerCase() == "d") {
            this.state.charVec.x += 15;
        }
    }
}
