import { Game } from "./game.js";
import { Popup } from "./popup.js";
import { renderAbilityBar } from "./abilities.js";
import { WordleBoard } from "./wordle-board.js";
import { ClientWordleGuess, ClientWordleStart, WordleLose, WordleReq, WordleRes, WordleResume, WordleWin } from "./protocol.gen.js";

const GUESSES = 5;
// TODO: get word length from backend!!!
const WORD_LENGTH = 5;

// Seconds to m:ss
export function formatTime(secs: number): string {
    const total = Math.max(0, Math.floor(secs));
    const min = Math.floor(total / 60);
    const sec = total % 60;
    return min + ":" + (sec < 10 ? "0" : "") + sec;
}

// A m:ss clock in el, display only: the server keeps the real time
export class Stopwatch {
    private timer: number | null = null;

    constructor(private el: HTMLElement) {}

    // Counts up from alreadyPlayed seconds
    public start(alreadyPlayed = 0) {
        this.stop();
        const start = Date.now() - alreadyPlayed * 1000;
        const tick = () => {
            this.el.textContent = formatTime((Date.now() - start) / 1000);
        };
        tick();
        this.timer = setInterval(tick, 1000);
    }

    public stop() {
        if (this.timer !== null) {
            clearInterval(this.timer);
            this.timer = null;
        }
    }
}

// The word as a row of flipping tiles, green if it was found and grey if not
export function renderSolution(el: HTMLElement, word: string, found: boolean) {
    el.classList.toggle("lost", !found);
    el.replaceChildren();
    [...word.toUpperCase()].forEach((ch, i) => {
        const tile = document.createElement("span");
        tile.textContent = ch;
        tile.style.setProperty("--delay", i * 90 + "ms");
        el.appendChild(tile);
    });
}

export class Wordle {
    game: Game;
    popup: Popup | null;
    board: WordleBoard | null;
    stopwatch: Stopwatch | null;

    constructor(game: Game) {
        this.game = game;
        this.popup = null;
        this.board = null;
        this.stopwatch = null;
    }

    // Opens the wordle. Returns false if another popup is already open.
    public run(): boolean {
        const popup = Popup.open("tpl-wordle-game", this.game.inputDriver);
        if (!popup) {
            return false;
        }
        this.popup = popup;
        this.stopwatch = new Stopwatch(popup.q("#timer"));
        popup.onCleanup(() => this.stopwatch?.stop());
        popup.onClose = () => {
            if (this.game.wordle === this) {
                this.game.wordle = null;
            }
        };

        const board = new WordleBoard(popup, popup.q("#gameContainer"), popup.q("#submit"), { wordLength: WORD_LENGTH, rows: GUESSES });
        // The server's reply to starting has arrived. Guesses wait for it, or the
        // reply (sent before the guess was counted) would reset the row.
        board.waitUntilReady();
        board.onSubmit = (guess) => {
            const data: WordleReq = { guess };
            this.game.send(ClientWordleGuess, data);
        };
        this.board = board;
        // Empty for now, abilities will be usable during the puzzle
        renderAbilityBar(popup.q("#abilityBar"), this.game.state.selfClass);

        // Server starts the clock and replies with any guesses already made
        this.game.send(ClientWordleStart, {});
        return true;
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
        this.board?.restore(resume.guesses ?? [], resume.colors ?? []);
        this.stopwatch?.start(resume.seconds);
        this.board?.setReady();
    }

    public handleResponse(res: WordleRes) {
        const board = this.board;
        if (!board) {
            return;
        }
        if (!res.valid) {
            board.reject();
            this.game.toast("Not in word list");
            return;
        }
        board.colorRow(board.currentGuess - 1, res.colors);

        if (res.status == WordleWin) {
            this.displayResult(true, res.solution, res.seconds);
        } else if (res.status == WordleLose) {
            this.displayResult(false, res.solution, res.seconds);
        }
    }

    private displayResult(win: boolean, word: string, seconds: number) {
        this.stopwatch?.stop();
        const guesses = this.board?.currentGuess ?? 0;
        const layer = this.popup!.addLayer("tpl-wordle-result");

        const set = (id: string, text: string) => layer.querySelector("#" + id)!.textContent = text;
        if (win) {
            set("resultTitle", "Solved");
            const plural = guesses == 1 ? "guess" : "guesses";
            set("resultText", "You got it in " + guesses + " " + plural + ".");
            set("resultGuesses", guesses + "/" + GUESSES);
        } else {
            set("resultTitle", "Out of guesses");
            set("resultText", "Better luck tomorrow.");
            set("resultGuesses", "X/" + GUESSES);
        }
        set("resultTime", formatTime(seconds));
        renderSolution(layer.querySelector<HTMLDivElement>("#solutionText")!, word, win);
    }
}
