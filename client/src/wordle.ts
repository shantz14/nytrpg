import { Game } from "./game.js";
import { Popup } from "./popup.js";
import { ClientWordleGuess, ClientWordleStart, Green, Grey, WordleColor, WordleLose, WordleReq, WordleRes, WordleResume, WordleWin, Yellow } from "./protocol.gen.js";

const GUESSES = 5;

function letterId(row: number, col: number): string {
    return `letter-${row}-${col}`;
}

// Seconds to m:ss
export function formatTime(secs: number): string {
    const total = Math.max(0, Math.floor(secs));
    const min = Math.floor(total / 60);
    const sec = total % 60;
    return min + ":" + (sec < 10 ? "0" : "") + sec;
}

export class Wordle {
    game: Game;
    popup: Popup | null;
    wordLength: number;
    nextLetter: HTMLInputElement | null;
    currentGuess: number;
    stopwatch: number | null;
    // The server's reply to starting has arrived. Guesses wait for it, or the
    // reply (sent before the guess was counted) would reset currentGuess.
    ready: boolean;
    submitWhenReady: boolean;

    constructor(game: Game) {
        this.game = game;
        this.popup = null;
        this.wordLength = this.getWordLength();
        this.nextLetter = null;
        this.currentGuess = 0;
        this.stopwatch = null;
        this.ready = false;
        this.submitWhenReady = false;
    }

    // Opens the wordle. Returns false if another popup is already open.
    public run(): boolean {
        const popup = Popup.open("tpl-wordle-game", this.game.inputDriver);
        if (!popup) {
            return false;
        }
        this.popup = popup;
        popup.onCleanup(() => this.stopStopwatch());
        popup.onClose = () => {
            if (this.game.wordle === this) {
                this.game.wordle = null;
            }
        };

        this.displayGame(popup);
        this.populateGame(popup);

        // Server starts the clock and replies with any guesses already made
        this.game.send(ClientWordleStart, {});
        return true;
    }

    // Timer display only, the server keeps the real time
    private runStopwatch(alreadyPlayed: number) {
        this.stopStopwatch();
        const start = Date.now() - alreadyPlayed * 1000;
        const timer = this.popup!.q<HTMLDivElement>("#timer");
        const tick = () => {
            timer.textContent = formatTime((Date.now() - start) / 1000);
        };
        tick();
        this.stopwatch = setInterval(tick, 1000);
    }

    private stopStopwatch() {
        if (this.stopwatch !== null) {
            clearInterval(this.stopwatch);
            this.stopwatch = null;
        }
    }

    // TODO: get word length from backend!!!
    private getWordLength(): number {
        return 5;
    }

    private getLetter(row: number, col: number): HTMLInputElement | null {
        return this.popup?.root.querySelector("#" + letterId(row, col)) ?? null;
    }

    private displayGame(popup: Popup) {
        const submit = popup.q<HTMLButtonElement>("#submit");
        popup.on(submit, "click", () => {
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
                this.sendGuess(guess);
                this.nextLetter?.focus();
            }
        });

        popup.on(document, "keypress", (e) => {
            if ((e as KeyboardEvent).key === "Enter") {
                e.preventDefault();
                submit.click();
            }
        });
        popup.on(document, "keydown", (e) => {
            if ((e as KeyboardEvent).key === "Backspace") {
                e.preventDefault();
                this.cancelMove();
            }
        });
    }

    private sendGuess(guess: string) {
        const data: WordleReq = {
            guess: guess,
        };

        this.game.send(ClientWordleGuess, data);
    }

    // Fill in guesses made before a reload and pick the timer back up
    public handleResume(resume: WordleResume) {
        if (resume.played) {
            this.popup?.addLayer("tpl-wordle-played");
            return;
        }
        if (resume.tooFar) {
            this.popup?.close();
            this.game.toast("Walk closer to use that");
            return;
        }
        const guesses = resume.guesses ?? [];
        const colors = resume.colors ?? [];
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
        this.runStopwatch(resume.seconds);
        this.ready = true;
        if (guesses.length > 0) {
            this.getLetter(this.currentGuess, 0)?.focus();
        }
        if (this.submitWhenReady) {
            this.submitWhenReady = false;
            this.popup?.q<HTMLButtonElement>("#submit").click();
        }
    }

    public handleResponse(res: WordleRes) {
        if (!res.valid) {
            this.currentGuess--;
            this.cancelMove();
            return;
        }
        this.colorRow(this.currentGuess - 1, res.colors);

        if (res.status == WordleWin) {
            this.displayResult(true, res.solution, res.seconds);
        } else if (res.status == WordleLose) {
            this.displayResult(false, res.solution, res.seconds);
        }
    }

    private cancelMove() {
        this.getLetter(this.currentGuess, 0)?.focus();
        for (let i = 0; i < this.wordLength; i++) {
            const letter = this.getLetter(this.currentGuess, i);
            if (letter) {
                letter.value = "";
            }
        }
    }

    private displayResult(win: boolean, word: string, seconds: number) {
        this.stopStopwatch();
        const layer = this.popup!.addLayer("tpl-wordle-result");

        const resultText = layer.querySelector("#resultText")!;
        if (win) {
            const plural = this.currentGuess > 1 ? " Guesses!" : " Guess!";
            resultText.textContent = "You Won In " + this.currentGuess + plural + " (" + formatTime(seconds) + ")";
        } else {
            resultText.textContent = "You Lose...";
        }
        layer.querySelector("#solutionText")!.textContent = word;
    }

    private colorRow(row: number, colors: Array<WordleColor>) {
        for (let i = 0; i < this.wordLength; i++) {
            const box = this.getLetter(row, i);
            if (!box) {
                continue;
            }
            if (colors[i] == Grey) {
                box.style.backgroundColor = "grey";
            } else if (colors[i] == Yellow) {
                box.style.backgroundColor = "yellow";
            } else if (colors[i] == Green) {
                box.style.backgroundColor = "green";
            }
        }
    }

    private populateGame(popup: Popup) {
        const gameContainer = popup.q<HTMLDivElement>("#gameContainer");
        const wordContainer = popup.q<HTMLDivElement>("#wordContainer0");
        const template = popup.q<HTMLInputElement>("#" + letterId(0, 0));
        template.remove();

        for (let r = 1; r < GUESSES; r++) {
            const newWord = wordContainer.cloneNode() as HTMLDivElement;
            newWord.id = "wordContainer" + r;
            this.populateWord(newWord, r, template);
            gameContainer.appendChild(newWord);
        }
        this.populateWord(wordContainer, 0, template);

        this.getLetter(0, 0)?.focus();
    }

    private populateWord(wordContainer: HTMLDivElement, row: number, template: HTMLInputElement) {
        for (let c = 0; c < this.wordLength; c++) {
            const newLetter = template.cloneNode(true) as HTMLInputElement;
            newLetter.id = letterId(row, c);
            newLetter.dataset.row = String(row);
            newLetter.dataset.col = String(c);

            newLetter.addEventListener("input", () => {
                this.validateInput(newLetter);
            });

            wordContainer.appendChild(newLetter);
        }
    }

    private validateInput(letter: HTMLInputElement) {
        if (/^[a-zA-Z]$/.test(letter.value)) {
            letter.value = letter.value.toUpperCase();
        } else {
            letter.value = "";
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
    }
}
