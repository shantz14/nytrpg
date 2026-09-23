import { Game } from "./game.js";
import { ClientWordleGuess, ClientWordleStart, Green, Grey, WordleColor, WordleLose, WordleReq, WordleRes, WordleResume, WordleWin, Yellow } from "./protocol.gen.js";

const GUESSES = 5;
const wordleURL = "/haveIPlayed"

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
    wordLength: number;
    submitButton: HTMLButtonElement | null;
    nextLetter: HTMLInputElement | null;
    currentGuess: number;
    stopwatch: number | null;
    onKeypress: ((e: KeyboardEvent) => void) | null;
    onKeydown: ((e: KeyboardEvent) => void) | null;

    constructor(game: Game) {
        this.game = game;
        this.wordLength = this.getWordLength();
        this.submitButton = null;
        this.nextLetter = null;
        this.currentGuess = 0;
        this.stopwatch = null;
        this.onKeypress = null;
        this.onKeydown = null;
    }

    public run() {
        this.game.inputDriver.setPopupFocused();

        this.haveIPlayedToday()
        .then(played => {
            if (played) {
                this.displayYouHavePlayed();
                return;
            } else {
                this.displayGame();
                this.populateGame();
                // Server starts the clock and replies with any guesses already made
                this.game.send(ClientWordleStart, {});
            }
        });
    }

    private async haveIPlayedToday(): Promise<boolean> {
        const options = {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            },
        }
        return fetch(wordleURL + `?id=${this.game.userData.id}`, options)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error getting wordle thingy. Status: ${response.status}`);
            }
            return response.json();
        })
        .then(responseData => {
            return responseData;
        })
        .catch(error => {
            console.error('Error parsing haveIPlayedToday:', error);
            return true;
        });
    }

    private displayYouHavePlayed() {
        const tpl = document.getElementById("tpl-wordle-played") as HTMLTemplateElement;
        document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));

        const exitButton = document.getElementById("exit") as HTMLButtonElement;
        exitButton.addEventListener("click", () => {
            document.getElementById("resultPopup")!.remove();
            this.game.inputDriver.setGameFocused();
            this.game.wordle = null;
        });
    }

    // Timer display only, the server keeps the real time
    private runStopwatch(alreadyPlayed: number) {
        this.stopStopwatch();
        const start = Date.now() - alreadyPlayed * 1000;
        const timer = document.getElementById("timer") as HTMLDivElement;
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
        return document.getElementById(letterId(row, col)) as HTMLInputElement | null;
    }

    private displayGame() {
        const tpl = document.getElementById("tpl-wordle-game") as HTMLTemplateElement;
        document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));

        const submit = document.getElementById("submit") as HTMLButtonElement;
        if (!submit) {
            console.error("Could not find submit button.");
            return;
        }
        this.submitButton = submit;

        this.submitButton.addEventListener("click", () => {
            let guess = "";

            for (let i = 0; i < this.wordLength; i++) {
                const box = this.getLetter(this.currentGuess, i);
                if (box) {
                    guess = guess + box.value;
                }
            }

            if (guess.length == this.wordLength) {
                this.currentGuess++;
                this.sendGuess(guess);

                if (this.nextLetter) {
                    this.nextLetter.focus();
                }
            }
        });

        this.onKeypress = (e: KeyboardEvent) => {
            if (e.key === "Enter" && !this.game.inputDriver.isGameFocused()) {
                e.preventDefault();
                this.submitButton?.click();
            }
        };
        this.onKeydown = (e: KeyboardEvent) => {
            if (e.key === "Backspace" && !this.game.inputDriver.isGameFocused()) {
                e.preventDefault();
                this.cancelMove();
            }
        };
        document.addEventListener("keypress", this.onKeypress);
        document.addEventListener("keydown", this.onKeydown);
    }

    private sendGuess(guess: string) {
        const data: WordleReq = {
            guess: guess,
        };

        this.game.send(ClientWordleGuess, data);
    }

    // Fill in guesses made before a reload and pick the timer back up
    public handleResume(resume: WordleResume) {
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
        this.getLetter(this.currentGuess, 0)?.focus();
        this.runStopwatch(resume.seconds);
    }

    public handleResponse(res: WordleRes) {
        if (!res.valid) {
            this.currentGuess--;
            this.cancelMove();
            return;
        }
        this.colorRow(this.currentGuess - 1, res.colors);

        if (res.status == WordleWin) {
            this.displayResultDiv(true, res.solution, res.seconds);
        } else if (res.status == WordleLose) {
            this.displayResultDiv(false, res.solution, res.seconds);
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

    private deleteGame() {
        document.getElementById("resultPopup")?.remove();
        document.getElementById("wordlePopup")?.remove();
        this.stopStopwatch();
        if (this.onKeypress) {
            document.removeEventListener("keypress", this.onKeypress);
        }
        if (this.onKeydown) {
            document.removeEventListener("keydown", this.onKeydown);
        }
        this.game.inputDriver.setGameFocused();
        this.game.wordle = null;
    }

    private displayResultDiv(win: boolean, word: string, seconds: number) {
        this.stopStopwatch();
        const tpl = document.getElementById("tpl-wordle-result") as HTMLTemplateElement;
        document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));

        const exitButton = document.getElementById("exit") as HTMLButtonElement;
        exitButton.addEventListener("click", () => {
            this.deleteGame();
        });

        const resultText = document.getElementById("resultText");
        if (resultText) {
            if (win) {
                const plural = this.currentGuess > 1 ? " Guesses!" : " Guess!";
                resultText.textContent = "You Won In " + this.currentGuess + plural + " (" + formatTime(seconds) + ")";
            } else {
                resultText.textContent = "You Lose...";
            }
        } else {
            console.log("No resultText element found.");
        }
        const solutionText = document.getElementById("solutionText");
        if (solutionText) {
            solutionText.textContent = word;
        } else {
            console.log("No solutionText element found.");
        }
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

    private populateGame() {
        const gameContainer = document.getElementById("gameContainer") as HTMLDivElement;
        const wordContainer = document.getElementById("wordContainer0") as HTMLDivElement;
        const template = this.getLetter(0, 0) as HTMLInputElement;
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
