import { Popup } from "./popup.js";
import { Green, Grey, WordleColor, Yellow } from "./protocol.gen.js";

export type BoardOptions = {
    wordLength: number;
    rows: number;
    // Prefixes element ids, so two boards can share a page
    idPrefix?: string;
};

export function tileClass(c: WordleColor): string | null {
    return c == Green ? "tile-green" : c == Yellow ? "tile-yellow" : c == Grey ? "tile-grey" : null;
}

// The rows of letter boxes you type guesses into, used by the daily Wordle and
// duels. It only handles input and showing results: the owner sends guesses
// (onSubmit) and passes back what the server said.
export class WordleBoard {
    // The row being typed into
    currentGuess: number;
    onSubmit: (guess: string) => void;
    // How many letters are in the current row, called whenever that changes
    onTypingChange: ((count: number) => void) | null;
    private opts: Required<BoardOptions>;
    private container: HTMLElement;
    private submitBtn: HTMLButtonElement;
    private nextLetter: HTMLInputElement | null;
    // Guesses wait for ready, see setReady
    private ready: boolean;
    private submitWhenReady: boolean;
    private locked: boolean;
    private lastTyped: number;

    constructor(popup: Popup, container: HTMLElement, submitBtn: HTMLButtonElement, opts: BoardOptions) {
        this.opts = { idPrefix: "", ...opts };
        this.container = container;
        this.submitBtn = submitBtn;
        this.currentGuess = 0;
        this.onSubmit = () => {};
        this.onTypingChange = null;
        this.nextLetter = null;
        this.ready = true;
        this.submitWhenReady = false;
        this.locked = false;
        this.lastTyped = 0;

        this.build();
        popup.on(submitBtn, "click", () => this.submit());
        popup.on(document, "keypress", (e) => {
            if ((e as KeyboardEvent).key === "Enter") {
                e.preventDefault();
                this.submitBtn.click();
            }
        });
        popup.on(document, "keydown", (e) => {
            if ((e as KeyboardEvent).key === "Backspace") {
                e.preventDefault();
                this.cancelMove();
            }
        });
    }

    get wordLength(): number {
        return this.opts.wordLength;
    }

    // Holds guesses until setReady, e.g. while waiting for the server to say
    // which row we're on. A guess submitted meanwhile goes out once ready.
    public waitUntilReady() {
        this.ready = false;
    }

    public setReady() {
        this.ready = true;
        if (this.submitWhenReady) {
            this.submitWhenReady = false;
            this.submit();
        }
    }

    // No more typing or guessing, e.g. once the game is over
    public lock() {
        this.locked = true;
        this.submitBtn.disabled = true;
        for (const box of this.container.querySelectorAll<HTMLInputElement>("input.letter")) {
            box.readOnly = true;
            box.blur();
        }
        this.markActiveRow();
    }

    // Puts the cursor in the current row
    public focus() {
        if (!this.locked) {
            this.getLetter(this.currentGuess, 0)?.focus();
        }
    }

    public getLetter(row: number, col: number): HTMLInputElement | null {
        return this.container.querySelector("#" + this.opts.idPrefix + `letter-${row}-${col}`);
    }

    private row(row: number): HTMLDivElement | null {
        return this.container.querySelector("#" + this.opts.idPrefix + "wordContainer" + row);
    }

    private submit() {
        if (this.locked) {
            return;
        }
        if (!this.ready) {
            this.submitWhenReady = true;
            return;
        }
        let guess = "";
        for (let i = 0; i < this.wordLength; i++) {
            guess += this.getLetter(this.currentGuess, i)?.value ?? "";
        }
        if (guess.length == this.wordLength) {
            this.currentGuess++;
            this.markActiveRow();
            this.onSubmit(guess);
            this.nextLetter?.focus();
            this.emitTyping();
        }
    }

    // Fills in guesses already made, e.g. after a reload
    public restore(guesses: string[], colors: WordleColor[][]) {
        for (let row = 0; row < guesses.length; row++) {
            for (let col = 0; col < this.wordLength; col++) {
                const box = this.getLetter(row, col);
                if (box) {
                    box.value = guesses[row][col] ?? "";
                }
            }
            this.colorRow(row, colors[row] ?? []);
        }
        this.currentGuess = guesses.length;
        this.markActiveRow();
        if (guesses.length > 0) {
            this.getLetter(this.currentGuess, 0)?.focus();
        }
    }

    // The last guess wasn't a word: take it back so it can be retyped
    public reject() {
        this.currentGuess--;
        this.markActiveRow();
        this.cancelMove();
        this.shakeRow(this.currentGuess);
    }

    public colorRow(row: number, colors: WordleColor[]) {
        for (let i = 0; i < this.wordLength; i++) {
            const box = this.getLetter(row, i);
            const tile = tileClass(colors[i]);
            if (!box || !tile) {
                continue;
            }
            box.classList.add("revealed", tile);
            box.style.setProperty("--delay", i * 120 + "ms");
        }
    }

    // Highlights the row the player is typing into
    private markActiveRow() {
        for (let r = 0; r < this.opts.rows; r++) {
            this.row(r)?.classList.toggle("active", !this.locked && r == this.currentGuess);
        }
    }

    private shakeRow(row: number) {
        const el = this.row(row);
        if (!el) {
            return;
        }
        el.classList.remove("shake");
        void el.offsetWidth; // restart the animation
        el.classList.add("shake");
    }

    // Clears the current row
    private cancelMove() {
        if (this.locked) {
            return;
        }
        this.getLetter(this.currentGuess, 0)?.focus();
        for (let i = 0; i < this.wordLength; i++) {
            const letter = this.getLetter(this.currentGuess, i);
            if (letter) {
                letter.value = "";
            }
        }
        this.emitTyping();
    }

    private emitTyping() {
        let count = 0;
        for (let i = 0; i < this.wordLength; i++) {
            if (this.getLetter(this.currentGuess, i)?.value) {
                count++;
            }
        }
        if (count !== this.lastTyped) {
            this.lastTyped = count;
            this.onTypingChange?.(count);
        }
    }

    private build() {
        this.container.replaceChildren();
        for (let r = 0; r < this.opts.rows; r++) {
            const row = document.createElement("div");
            row.className = "wordContainer";
            row.id = this.opts.idPrefix + "wordContainer" + r;
            for (let c = 0; c < this.wordLength; c++) {
                row.appendChild(this.letterBox(r, c));
            }
            this.container.appendChild(row);
        }
        this.markActiveRow();
        this.getLetter(0, 0)?.focus();
    }

    private letterBox(row: number, col: number): HTMLInputElement {
        const box = document.createElement("input");
        box.type = "text";
        box.className = "letter";
        box.id = this.opts.idPrefix + `letter-${row}-${col}`;
        box.maxLength = 1;
        box.autocomplete = "off";
        box.spellcheck = false;
        box.placeholder = " ";
        box.dataset.row = String(row);
        box.dataset.col = String(col);
        box.addEventListener("input", () => this.validateInput(box));
        return box;
    }

    private validateInput(letter: HTMLInputElement) {
        if (/^[a-zA-Z]$/.test(letter.value)) {
            letter.value = letter.value.toUpperCase();
        } else {
            letter.value = "";
            this.emitTyping();
            return;
        }

        const row = Number(letter.dataset.row);
        const col = Number(letter.dataset.col);

        // End of a row points at the start of the next one, focused after submitting
        const next = col == this.wordLength - 1 ? this.getLetter(row + 1, 0) : this.getLetter(row, col + 1);
        this.nextLetter = next;
        if (next && Number(next.dataset.row) == this.currentGuess) {
            next.focus();
        }
        this.emitTyping();
    }
}

// The opponent's board in a duel: colors only, never letters. Their current row
// shows how many letters they've typed as plain filled boxes.
export class OpponentGrid {
    guesses: number;
    solved: boolean;
    private rows: HTMLDivElement[];

    constructor(container: HTMLElement, private wordLength: number, private maxGuesses: number) {
        this.guesses = 0;
        this.solved = false;
        this.rows = [];
        container.replaceChildren();
        for (let r = 0; r < maxGuesses; r++) {
            const row = document.createElement("div");
            row.className = "opp-row";
            for (let c = 0; c < wordLength; c++) {
                const tile = document.createElement("div");
                tile.className = "opp-tile";
                row.appendChild(tile);
            }
            container.appendChild(row);
            this.rows.push(row);
        }
        this.markActiveRow();
    }

    get done(): boolean {
        return this.solved || this.guesses >= this.maxGuesses;
    }

    public addGuess(colors: WordleColor[]) {
        const row = this.rows[this.guesses];
        if (!row) {
            return;
        }
        [...row.children].forEach((tile, i) => {
            tile.classList.remove("typed");
            const cls = tileClass(colors[i]);
            if (cls) {
                tile.classList.add("revealed", cls);
                (tile as HTMLElement).style.setProperty("--delay", i * 120 + "ms");
            }
        });
        this.guesses++;
        this.solved = colors.length > 0 && colors.every((c) => c == Green);
        this.markActiveRow();
    }

    // Fills the first count boxes of the row they're typing in
    public setTyping(count: number) {
        const row = this.rows[this.guesses];
        if (!row || this.done) {
            return;
        }
        [...row.children].forEach((tile, i) => tile.classList.toggle("typed", i < count));
    }

    private markActiveRow() {
        this.rows.forEach((row, r) => row.classList.toggle("active", !this.done && r == this.guesses));
    }
}
