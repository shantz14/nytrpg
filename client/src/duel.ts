import { Game } from "./game.js";
import { Popup } from "./popup.js";
import { OpponentGrid, WordleBoard } from "./wordle-board.js";
import { Stopwatch, renderSolution } from "./wordle.js";
import { LatestThrottle } from "./throttle.js";
import { className } from "./classes.js";
import {
    ClientDuelForfeit, ClientDuelGuess, ClientDuelTyping, DuelDisconnect, DuelDraw, DuelEnd, DuelForfeit, DuelOpponentGuess,
    DuelOutOfGuesses, DuelSolved, DuelStart, DuelTyping, DuelWin, WordleLose, WordleRes, WordleWin,
} from "./protocol.gen.js";

// Typing updates go out at most this often
const TYPING_INTERVAL_MS = 50;

// A duel in progress: your board and the opponent's colors side by side
export class Duel {
    private game: Game;
    private start: DuelStart;
    private popup: Popup | null;
    private board: WordleBoard | null;
    private opponent: OpponentGrid | null;
    private stopwatch: Stopwatch | null;
    private typing: LatestThrottle<number>;
    private ended: boolean;

    constructor(game: Game, start: DuelStart) {
        this.game = game;
        this.start = start;
        this.popup = null;
        this.board = null;
        this.opponent = null;
        this.stopwatch = null;
        this.ended = false;
        this.typing = new LatestThrottle<number>((count) => {
            const t: DuelTyping = { count };
            this.game.send(ClientDuelTyping, t);
        }, TYPING_INTERVAL_MS);
    }

    // The opponent: character name, or username if they have none
    private get opponentName(): string {
        return this.start.char || this.start.name;
    }

    // Opens the duel window, closing whatever popup was open
    public open() {
        Popup.current?.close();
        const popup = Popup.open("tpl-duel", this.game.inputDriver, false)!;
        this.popup = popup;
        popup.onCleanup(() => {
            this.stopwatch?.stop();
            this.typing.cancel();
        });
        popup.onClose = () => {
            if (this.game.duel === this) {
                this.game.duel = null;
            }
        };

        const s = this.start;
        popup.q("#duelVsName").textContent = this.opponentName;
        const cls = popup.q("#duelVsClass");
        cls.textContent = s.class ? className(this.game.state.classes, s.class) : "";
        cls.className = "duel-vs-class" + (s.class ? " class-" + s.class : "");
        popup.q("#duelOppLabel").textContent = this.opponentName;

        const board = new WordleBoard(popup, popup.q("#duelBoard"), popup.q("#duelSubmit"), {
            wordLength: s.wordLength,
            rows: s.maxGuesses,
            idPrefix: "duel-",
        });
        board.onSubmit = (guess) => this.game.send(ClientDuelGuess, { guess });
        board.onTypingChange = (count) => this.typing.push(count);
        this.board = board;
        board.focus();
        this.opponent = new OpponentGrid(popup.q("#duelOpponent"), s.wordLength, s.maxGuesses);
        this.updateOpponentStatus();

        this.stopwatch = new Stopwatch(popup.q("#duelTimer"));
        this.stopwatch.start();

        popup.on(popup.q("#duelForfeit"), "click", () => this.confirmForfeit());
    }

    private confirmForfeit() {
        const popup = this.popup;
        if (!popup || this.ended || document.getElementById("duelForfeitPopup")) {
            return;
        }
        const layer = popup.addLayer("tpl-duel-forfeit");
        layer.querySelector("#forfeitName")!.textContent = this.opponentName;
        const close = () => {
            popup.removeLayer(layer);
            this.board?.focus();
        };
        layer.querySelector("#cancelForfeit")!.addEventListener("click", close);
        layer.querySelector("#confirmForfeit")!.addEventListener("click", () => {
            close();
            this.game.send(ClientDuelForfeit, {});
        });
    }

    public handleGuess(res: WordleRes) {
        const board = this.board;
        if (!board || this.ended) {
            return;
        }
        if (!res.valid) {
            board.reject();
            this.game.toast("Not in word list");
            return;
        }
        board.colorRow(board.currentGuess - 1, res.colors);
        if (res.status == WordleLose) {
            board.lock();
            this.setYouStatus("Out of guesses. Waiting for " + this.opponentName + "…");
        } else if (res.status == WordleWin) {
            board.lock();
        }
    }

    public handleOpponentGuess(g: DuelOpponentGuess) {
        this.opponent?.addGuess(g.colors);
        this.updateOpponentStatus();
    }

    public handleTyping(t: DuelTyping) {
        this.opponent?.setTyping(t.count);
    }

    private updateOpponentStatus() {
        const opp = this.opponent;
        if (!opp || !this.popup) {
            return;
        }
        const status = this.popup.q("#duelOppStatus");
        status.textContent = opp.solved ? "Solved" : opp.done ? "Out of guesses" : `${opp.guesses}/${this.start.maxGuesses}`;
        status.classList.toggle("solved", opp.solved);
        status.classList.toggle("out", opp.done && !opp.solved);
    }

    private setYouStatus(text: string) {
        this.popup?.q("#duelYouStatus").replaceChildren(text);
    }

    public handleEnd(end: DuelEnd) {
        const popup = this.popup;
        if (!popup || this.ended) {
            return;
        }
        this.ended = true;
        this.stopwatch?.stop();
        this.typing.cancel();
        this.board?.lock();
        popup.q<HTMLButtonElement>("#duelForfeit").disabled = true;

        const layer = popup.addLayer("tpl-duel-result");
        const set = (id: string, text: string) => layer.querySelector("#" + id)!.textContent = text;
        const name = this.opponentName;
        const title = end.outcome == DuelWin ? "Victory" : end.outcome == DuelDraw ? "Draw" : "Defeat";
        layer.querySelector(".result-panel")!.classList.add("duel-" + title.toLowerCase());
        set("duelResultTitle", title);

        let text: string;
        if (end.reason == DuelForfeit) {
            text = end.outcome == DuelWin ? `${name} gave up.` : `You gave up against ${name}.`;
        } else if (end.reason == DuelDisconnect) {
            text = end.outcome == DuelWin ? `${name} left the duel.` : "You left the duel.";
        } else if (end.reason == DuelOutOfGuesses) {
            text = `Neither of you found the word.`;
        } else {
            text = end.outcome == DuelWin ? `You found the word before ${name}.` : `${name} found the word first.`;
        }
        set("duelResultText", text);
        // Green if someone found it
        renderSolution(layer.querySelector<HTMLDivElement>("#duelSolution")!, end.solution, end.reason == DuelSolved);
    }

    // The connection dropped: the server has already counted it as a loss
    public abandon() {
        this.popup?.close();
        if (!this.ended) {
            this.game.toast("Your duel ended when you lost connection");
        }
    }
}
