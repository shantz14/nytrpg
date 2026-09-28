import { Popup } from "./popup.js";
import { EyeTile, Green, Grey, Hidden, WordleColor, Yellow } from "./protocol.gen.js";
import { remapKey } from "./abilities.js";

export type BoardOptions = {
    wordLength: number;
    rows: number;
    // Prefixes element ids, so two boards can share a page
    idPrefix?: string;
};

const TILE_CLASSES = ["tile-green", "tile-yellow", "tile-grey", "tile-destroyed"];

// How long a lost row shows its crack before it crumbles away
export const CRACK_MS = 900;

export function tileClass(c: WordleColor): string | null {
    return c == Green ? "tile-green" : c == Yellow ? "tile-yellow" : c == Grey ? "tile-grey" : c == Hidden ? "tile-destroyed" : null;
}

// Cracks a row that was lost, then removes it
function crumble(row: HTMLElement) {
    row.classList.add("cracked");
    row.id = "";
    setTimeout(() => row.remove(), CRACK_MS);
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
    // For now, e.g. stunned. Unlike locked, it ends.
    private disabled: boolean;
    private lastTyped: number;
    // What each key types while scrambled (Confuse), "" when it isn't
    private keymap: string;
    private pickingRow: ((row: number | null) => void) | null;

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
        this.disabled = false;
        this.lastTyped = 0;
        this.keymap = "";
        this.pickingRow = null;

        this.build();
        popup.on(container, "click", (e) => {
            const row = (e.target as HTMLElement).closest<HTMLElement>(".wordContainer.pickable");
            if (row && this.pickingRow) {
                this.finishPickRow(Number(row.dataset.row));
            }
        });
        popup.on(submitBtn, "click", () => this.submit());
        popup.on(document, "keypress", (e) => {
            if ((e as KeyboardEvent).key === "Enter" && !this.inactive) {
                e.preventDefault();
                this.submitBtn.click();
            }
        });
        popup.on(document, "keydown", (e) => {
            if ((e as KeyboardEvent).key === "Backspace" && !this.inactive) {
                e.preventDefault();
                this.cancelMove();
            }
        });
    }

    get wordLength(): number {
        return this.opts.wordLength;
    }

    get rows(): number {
        return this.opts.rows;
    }

    get isLocked(): boolean {
        return this.locked;
    }

    private get inactive(): boolean {
        return this.locked || this.disabled;
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
        this.setReadOnly(true);
        this.markActiveRow();
    }

    // Guessing again after a lock, e.g. a duel gave you another row
    public unlock() {
        this.locked = false;
        this.setReadOnly(this.disabled);
        this.markActiveRow();
        this.focus();
    }

    // Stops typing and guessing for a while (stunned), or lets it go again
    public setDisabled(disabled: boolean) {
        if (this.disabled === disabled) {
            return;
        }
        this.disabled = disabled;
        this.container.classList.toggle("disabled", disabled);
        if (!this.locked) {
            this.setReadOnly(disabled);
            if (!disabled) {
                this.focus();
            }
        }
    }

    private setReadOnly(readOnly: boolean) {
        this.submitBtn.disabled = readOnly;
        for (const box of this.container.querySelectorAll<HTMLInputElement>("input.letter")) {
            box.readOnly = readOnly;
            if (readOnly) {
                box.blur();
            }
        }
    }

    // Puts the cursor in the current row
    public focus() {
        if (!this.inactive) {
            this.getLetter(this.currentGuess, 0)?.focus();
        }
    }

    public getLetter(row: number, col: number): HTMLInputElement | null {
        return this.container.querySelector("#" + this.opts.idPrefix + `letter-${row}-${col}`);
    }

    public row(row: number): HTMLDivElement | null {
        return this.container.querySelector("#" + this.opts.idPrefix + "wordContainer" + row);
    }

    private submit() {
        if (this.inactive) {
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

    // The last guess wasn't taken: take it back so it can be retyped
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
            if (!box || !tile || box.classList.contains("tile-destroyed")) {
                continue;
            }
            box.classList.remove(...TILE_CLASSES);
            box.classList.add("revealed", tile);
            box.style.setProperty("--delay", i * 120 + "ms");
        }
    }

    // Every guessed row's colors changed, e.g. the word did
    public recolor(colors: WordleColor[][]) {
        colors.forEach((row, r) => {
            this.colorRow(r, row);
            this.row(r)?.classList.remove("reshaped");
            void this.row(r)?.offsetWidth;
            this.row(r)?.classList.add("reshaped");
        });
    }

    // A letter is gone: no letter, no color
    public destroyTile(row: number, col: number) {
        const box = this.getLetter(row, col);
        if (!box) {
            return;
        }
        box.value = "";
        box.placeholder = "";
        box.classList.remove(...TILE_CLASSES);
        box.classList.add("revealed", "tile-destroyed");
        box.readOnly = true;
    }

    // Scrambles the keyboard: key i types keymap[i]. "" unscrambles it.
    public setKeymap(keymap: string) {
        this.keymap = keymap;
        this.container.classList.toggle("scrambled", keymap !== "");
    }

    // Takes back a guess (Mend): the rows below it, and whatever is typed in
    // the current row, move up one
    public removeGuess(row: number) {
        if (row < 0 || row >= this.currentGuess) {
            return;
        }
        for (let r = row; r < this.currentGuess; r++) {
            for (let c = 0; c < this.wordLength; c++) {
                const to = this.getLetter(r, c)!;
                const from = this.getLetter(r + 1, c);
                to.value = from?.value ?? "";
                to.className = from?.className ?? "letter";
                to.placeholder = from?.placeholder ?? " ";
                to.readOnly = from?.readOnly ?? false;
                to.style.setProperty("--delay", "0ms");
            }
        }
        for (let c = 0; c < this.wordLength; c++) {
            const box = this.getLetter(this.currentGuess, c);
            if (box) {
                box.value = "";
                box.className = "letter";
                box.placeholder = " ";
                box.readOnly = this.inactive;
            }
        }
        this.currentGuess--;
        this.markActiveRow();
        this.focus();
    }

    // Starts over with rows empty rows (Divine Intervention's clean slate)
    public reset(rows: number) {
        this.cancelPickRow();
        this.opts.rows = rows;
        this.currentGuess = 0;
        this.locked = false;
        this.lastTyped = 0;
        this.build();
        this.setReadOnly(this.disabled);
        this.focus();
    }

    // Lets the player click one of their guesses. null if cancelled.
    public pickRow(): Promise<number | null> {
        this.cancelPickRow();
        for (let r = 0; r < this.currentGuess; r++) {
            this.row(r)?.classList.add("pickable");
        }
        this.container.classList.add("targeting");
        return new Promise((resolve) => this.pickingRow = resolve);
    }

    public cancelPickRow() {
        this.finishPickRow(null);
    }

    get isPickingRow(): boolean {
        return this.pickingRow !== null;
    }

    private finishPickRow(row: number | null) {
        const resolve = this.pickingRow;
        this.pickingRow = null;
        this.container.classList.remove("targeting");
        for (const el of this.container.querySelectorAll(".pickable")) {
            el.classList.remove("pickable");
        }
        resolve?.(row);
    }

    // Adds rows at the bottom, or cracks and drops unused ones from it
    public setRows(rows: number) {
        while (this.opts.rows < rows) {
            const row = this.buildRow(this.opts.rows);
            row.classList.add("added");
            this.container.appendChild(row);
            this.opts.rows++;
            if (this.locked || this.disabled) {
                for (const box of row.querySelectorAll("input")) {
                    box.readOnly = true;
                }
            }
        }
        while (this.opts.rows > rows && this.opts.rows > this.currentGuess) {
            this.opts.rows--;
            const row = this.row(this.opts.rows);
            if (row) {
                crumble(row);
            }
        }
        this.markActiveRow();
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
        if (this.inactive) {
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
            this.container.appendChild(this.buildRow(r));
        }
        this.markActiveRow();
        this.getLetter(0, 0)?.focus();
    }

    private buildRow(r: number): HTMLDivElement {
        const row = document.createElement("div");
        row.className = "wordContainer";
        row.id = this.opts.idPrefix + "wordContainer" + r;
        row.dataset.row = String(r);
        for (let c = 0; c < this.wordLength; c++) {
            row.appendChild(this.letterBox(r, c));
        }
        return row;
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
            letter.value = remapKey(letter.value, this.keymap);
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

// The opponent's board in a duel: colors only, never letters (except the ones
// a Seeing Eye shows). Their current row shows how many letters they've typed
// as plain filled boxes.
export class OpponentGrid {
    guesses: number;
    solved: boolean;
    private container: HTMLElement;
    private rows: HTMLDivElement[];
    private picking: ((tile: { row: number; col: number } | null) => void) | null;

    constructor(container: HTMLElement, private wordLength: number, rows: number) {
        this.guesses = 0;
        this.solved = false;
        this.container = container;
        this.rows = [];
        this.picking = null;
        container.replaceChildren();
        for (let r = 0; r < rows; r++) {
            this.addRow();
        }
        this.markActiveRow();
        container.addEventListener("click", (e) => {
            const tile = (e.target as HTMLElement).closest<HTMLElement>(".opp-tile.pickable");
            if (tile && this.picking) {
                this.finishPick({ row: Number(tile.dataset.row), col: Number(tile.dataset.col) });
            }
        });
    }

    get rowCount(): number {
        return this.rows.length;
    }

    get done(): boolean {
        return this.solved || this.guesses >= this.rows.length;
    }

    public tile(row: number, col: number): HTMLElement | null {
        return (this.rows[row]?.children[col] as HTMLElement) ?? null;
    }

    private addRow() {
        const row = document.createElement("div");
        row.className = "opp-row";
        for (let c = 0; c < this.wordLength; c++) {
            const tile = document.createElement("div");
            tile.className = "opp-tile";
            tile.dataset.row = String(this.rows.length);
            tile.dataset.col = String(c);
            row.appendChild(tile);
        }
        this.container.appendChild(row);
        this.rows.push(row);
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

    // Every guessed row's colors changed, e.g. their word did
    public recolor(colors: WordleColor[][]) {
        colors.forEach((row, r) => {
            row.forEach((c, col) => {
                const tile = this.tile(r, col);
                const cls = tileClass(c);
                if (tile && cls && !tile.classList.contains("tile-destroyed")) {
                    tile.classList.remove(...TILE_CLASSES);
                    tile.classList.add(cls);
                }
            });
            const el = this.rows[r];
            el?.classList.remove("reshaped");
            void el?.offsetWidth;
            el?.classList.add("reshaped");
        });
    }

    public destroyTile(row: number, col: number) {
        const tile = this.tile(row, col);
        if (tile) {
            tile.classList.remove(...TILE_CLASSES, "eyed");
            tile.textContent = "";
            tile.classList.add("revealed", "tile-destroyed");
        }
    }

    // Adds rows at the bottom, or cracks and drops unused ones from it
    public setRows(rows: number) {
        while (this.rows.length < rows) {
            this.addRow();
            this.rows[this.rows.length - 1].classList.add("added");
        }
        while (this.rows.length > rows && this.rows.length > this.guesses) {
            crumble(this.rows.pop()!);
        }
        this.markActiveRow();
    }

    // Their guess was taken back (Mend): the rows below it move up one
    public removeGuess(row: number) {
        if (row < 0 || row >= this.guesses) {
            return;
        }
        for (let r = row; r <= this.guesses && r + 1 < this.rows.length; r++) {
            [...this.rows[r].children].forEach((tile, c) => {
                const from = this.rows[r + 1].children[c] as HTMLElement;
                tile.className = from.className;
                tile.textContent = from.textContent;
                (tile as HTMLElement).style.setProperty("--delay", "0ms");
            });
        }
        const last = this.rows[this.guesses];
        for (const tile of last?.children ?? []) {
            tile.className = "opp-tile";
            tile.textContent = "";
        }
        this.guesses--;
        this.solved = false;
        this.markActiveRow();
    }

    // Starts over with rows empty rows (Divine Intervention's clean slate)
    public reset(rows: number) {
        this.cancelPick();
        this.container.replaceChildren();
        this.rows = [];
        this.guesses = 0;
        this.solved = false;
        for (let r = 0; r < rows; r++) {
            this.addRow();
        }
        this.markActiveRow();
    }

    // A letter Pickpocket revealed, for good
    public showPickpocket(row: number, col: number, letter: string) {
        const tile = this.tile(row, col);
        if (tile) {
            tile.textContent = letter;
            tile.classList.add("picked");
        }
    }

    // Shows the letters Seeing Eyes see, in place of the old ones
    public showEyes(tiles: EyeTile[]) {
        for (const el of this.container.querySelectorAll(".opp-tile.eyed")) {
            el.classList.remove("eyed");
            if (!el.classList.contains("picked")) {
                el.textContent = "";
            }
        }
        for (const t of tiles) {
            const tile = this.tile(t.row, t.col);
            if (tile) {
                tile.textContent = t.letter;
                tile.classList.add("eyed");
            }
        }
    }

    // Lets the player click one of their guessed letters, only yellow ones if
    // yellowOnly. null if cancelled.
    public pickTile(yellowOnly = false): Promise<{ row: number; col: number } | null> {
        this.cancelPick();
        this.container.classList.add("targeting");
        for (let r = 0; r < this.guesses; r++) {
            for (const tile of this.rows[r].children) {
                if (!tile.classList.contains("tile-destroyed") && (!yellowOnly || tile.classList.contains("tile-yellow"))) {
                    tile.classList.add("pickable");
                }
            }
        }
        return new Promise((resolve) => this.picking = resolve);
    }

    public cancelPick() {
        this.finishPick(null);
    }

    get isPicking(): boolean {
        return this.picking !== null;
    }

    private finishPick(tile: { row: number; col: number } | null) {
        const resolve = this.picking;
        this.picking = null;
        this.container.classList.remove("targeting");
        for (const el of this.container.querySelectorAll(".pickable")) {
            el.classList.remove("pickable");
        }
        resolve?.(tile);
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
