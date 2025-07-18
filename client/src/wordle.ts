import { Game } from "./game.js";
import { ClientSendWordle, WordleReq, WordleResponse } from "./messages.js";
// TODO: wordle needs to be persistent after reload and keep counting the timer
// but have a timeout

const GUESSES = 5;
const TIME_LIMIT = 3; //minutes
const wordleURL = "/haveIPlayed"

export class Wordle {
    game: Game;
    wordLength: number;
    submitButton: HTMLButtonElement | null;
    nextLetter: HTMLInputElement | null;
    currentGuess: number;
    timePlayed: number;
    
    constructor(game: Game) {
        this.game = game;
        this.wordLength = this.getWordLength();
        this.submitButton = null;
        this.nextLetter = null;
        this.currentGuess = 0;
        this.timePlayed = 0;
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
                this.runStopwatch();
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
        const html = `
        <button id="exit">X</button>

        <div class="resultContainer" id="resultContainer">

            <p>You already played the wordle today.</p>

        </div>
        `;
        const popup = document.createElement("div");
        popup.setAttribute("id", "resultPopup")
        popup.innerHTML = html;

        const parent = document.getElementById("container");
        if (parent) {
            parent.appendChild(popup);
        } else {
            console.error("No parent to append popup to.");
        }

        const exitButton = document.getElementById("exit") as HTMLButtonElement;
        exitButton.addEventListener("click", () => {
            popup.remove();
            this.game.inputDriver.setGameFocused();
        });
    }

    private runStopwatch() {
        const start = Date.now();
        const timer = document.getElementById("timer") as HTMLDivElement;
        let minutes = 0;
        let seconds = 0;
        timer.innerHTML = minutes + ":" + seconds + "0";
        setInterval(() => {
            let delta = Date.now();

            if (seconds == 59) {
                timer.innerHTML = minutes + ":" + seconds;
                minutes++;
                seconds = 0;
            } else if (seconds < 10) {
                timer.innerHTML = minutes + ":" + "0" + seconds;
                seconds++;
            } else {
                timer.innerHTML = minutes + ":" + seconds;
                seconds++;
            }

            this.timePlayed = (delta - start) / 1000;
        }, 1000);
    }

    // TODO: get word length from backend!!!
    private getWordLength(): number {
        return 5;
    }

    private displayGame() {
        const gameHtml = `
        <div class="gameContainer" id="gameContainer">
            <div id="timer"></div>

            <div class="wordContainer" id="wordContainer0">
                <input type="text" class="letter" id="letter00" 
                maxlength="1" autocomplete="off">
            </div>

        </div>

        <button id="submit">Submit</button>
        `;
        const popup = document.createElement("div");
        popup.setAttribute("id", "wordlePopup");
        popup.innerHTML = gameHtml;

        const parent = document.getElementById("container");
        if (parent) {
            parent.appendChild(popup);
        } else {
            console.error("No parent to append popup to.");
        }

        const inputFunc = this.validateInput;
        const firstLetter = document.getElementById("letter00") as HTMLInputElement;
        firstLetter.addEventListener("input", (event) => {
            if (event instanceof InputEvent) {
                const newOne = event.target as HTMLInputElement;
                inputFunc(firstLetter, newOne.value, this);
            }
        });

        const submit = document.getElementById("submit") as HTMLButtonElement;
        if (!submit) {
            console.error("Could not find submit button.");
            return;
        }
        this.submitButton = submit;

        this.submitButton.addEventListener("click", () => {
            const baseId = "letter" + this.currentGuess;
            let guess = "";
            let boxes = [];

            for (let i = 0; i < this.wordLength; i++) {
                let box = document.getElementById(baseId + i) as HTMLInputElement;
                boxes.push(box);


                if (box) {
                    const letter = box.value;
                    guess = guess + letter;
                }
            }

            if (guess.length == this.wordLength) {
                this.currentGuess++;
                this.sendGuess(guess);
                
                if (this.nextLetter) {
                    this.nextLetter.focus();
                } else {
                    console.error("Could not find letter to focus on.");
                }
            }
        });

        document.addEventListener("keypress", (e: KeyboardEvent) => {
            if (e.key === "Enter" && !this.game.inputDriver.isGameFocused()) {
                e.preventDefault();
                this.submitButton?.click();
            }
        });
        document.addEventListener("keydown", (e: KeyboardEvent) => {
            if (e.key === "Backspace" && !this.game.inputDriver.isGameFocused()) {
                e.preventDefault();
                this.cancelMove();
            }
        });

    }

    private sendGuess(guess: string) {
        const data: WordleReq = {
            // TODO: make ids real
            id: -1,
            guess: guess,
            guessCount: this.currentGuess,
            time: Math.trunc(this.timePlayed)
        };

        this.game.send(ClientSendWordle, data);
    }

    public handleResponse(res: WordleResponse) {
        if (!res.valid) {
            this.currentGuess--;
            this.cancelMove();
            return;
        }
        const colors = res.colors;
        this.colorMyBoxes(colors);

        if (res.status == WIN) {
            this.win(res.solution);
        } else if (res.status == LOSE) {
            this.lose(res.solution);
        } else if (res.status == INGAME) {
            //notin
        }
    }

    private cancelMove() {
        const firstLetter = document.getElementById("letter" + (this.currentGuess) + "0");
        firstLetter?.focus();
        for (var i = 0; i < 5; i++) {
            const id = "letter" + (this.currentGuess) + i;
            const letter = document.getElementById(id) as HTMLInputElement;
            letter.value = "";
        }
    }

    private deleteGame() {
        const result = document.getElementById("resultPopup") as HTMLDivElement;
        result.remove();
        const popup = document.getElementById("wordlePopup") as HTMLDivElement;
        popup.remove();
        this.game.inputDriver.setGameFocused();
        this.game.wordle = null;
    }

    private win(word: string) {
        this.displayResultDiv(true, word);
    }

    private lose(word: string) {
        this.displayResultDiv(false, word);
    }

    private displayResultDiv(win: boolean, word: string) {
        const html = `
        <button id="exit">X</button>

        <div class="resultContainer" id="resultContainer">

            <p id="resultText"></p>
            <p>The word was:</p>
            <p id="solutionText"></p>

        </div>
        `;
        const popup = document.createElement("div");
        popup.setAttribute("id", "resultPopup")
        popup.innerHTML = html;

        const parent = document.getElementById("container");
        if (parent) {
            parent.appendChild(popup);
        } else {
            console.error("No parent to append popup to.");
        }

        const exitButton = document.getElementById("exit") as HTMLButtonElement;
        exitButton.addEventListener("click", () => {
            this.deleteGame();
        });

        const resultText = document.getElementById("resultText");
        if (resultText) {
            if (win) {
                if (this.currentGuess > 1) {
                    resultText.innerHTML = "You Won In " + this.currentGuess + " Guesses!" + " (" + this.secToMin(this.timePlayed) + ")";
                } else {
                    resultText.innerHTML = "You Won In " + this.currentGuess + " Guess!" + " (" + this.secToMin(this.timePlayed) + ")";
                }
            } else {
                resultText.innerHTML = "You Lose...";
            }
        } else {
            console.log("No resultText element found.");
        }
        const solutionText = document.getElementById("solutionText");
        if (solutionText) {
            solutionText.innerHTML = word;
        } else {
            console.log("No solutionText element found.");
        }
    }

    private secToMin(secs: number): string {
        const min = (secs / 60).toString()[0];
        const seconds = (secs % 60);
        var secStr = "";
        if (seconds < 10) {
            secStr = "0" + seconds.toFixed(0); 
        } else {
            secStr = seconds.toFixed(0);
        }
        const result = min + ":" + secStr;
        return result;
    }

    private colorMyBoxes(colors: Array<WordleColor>) {
        const baseId = "letter" + (this.currentGuess-1);

        for (let i = 0; i < 5; i++) {
            let box = document.getElementById(baseId + i) as HTMLInputElement;
            if (colors[i] == GREY) {
                box.style.backgroundColor = "grey";
            } else if (colors[i] == YELLOW) {
                box.style.backgroundColor = "yellow";
            } else if (colors[i] == GREEN) {
                box.style.backgroundColor = "green";
            }
        }
    }

    private populateGame() {
        let gameContainer = document.getElementById("gameContainer") as HTMLDivElement;
        let wordContainer = document.getElementById("wordContainer0") as  HTMLDivElement;

        for (let r = 1; r < GUESSES; r++) {
            let newWord = wordContainer.cloneNode() as HTMLDivElement;
            newWord.id = "wordContainer" + r;
            this.populateWord(newWord);
            gameContainer.appendChild(newWord);
        }
        this.populateWord(wordContainer);

        document.querySelector("input")?.remove();

        const letter0 = document.getElementById("letter00");
        if (letter0) {
            letter0.focus();
        }
    }

    private populateWord(wordContainer: HTMLDivElement) {
        const letterDiv = document.getElementById("letter00") as HTMLInputElement;
        for (let c = 0; c < this.wordLength; c++) {
            let newLetter = letterDiv.cloneNode(true) as HTMLInputElement;
            newLetter.id = "letter" + wordContainer.id[wordContainer.id.length - 1] + c;
            if (newLetter.id == "letter00") {
                letterDiv.remove();
            }

            console.log("adding: " + newLetter.id)
            const inputFunc = this.validateInput;
            newLetter.addEventListener("input", (event) => {
                if (event instanceof InputEvent) {
                    const newOne = event.target as HTMLInputElement;
                    inputFunc(newLetter, newOne.value, this);
                }
            });

            wordContainer.appendChild(newLetter);
        }
    }

    private validateInput(letter: HTMLInputElement, value: string, game: Wordle) {
        const charCode = value.charCodeAt(0);

        if (charCode < 65 || charCode > 122) {
            letter.value = "";
        } else {
            letter.value = value.toUpperCase();
        }

        const oldId = letter.id;
        var row = parseInt(oldId[oldId.length - 2]);
        var col = parseInt(oldId[oldId.length - 1]);

        if (col == game.wordLength - 1) {
            if (row == GUESSES - 1) {
                row = 0;
            } else {
                row++;
            }
            col = 0;
        } else {
            col++;
        }

        var newId = "letter" + row.toString() + col.toString();
        game.nextLetter = document.getElementById(newId) as HTMLInputElement;
        if (row == game.currentGuess) {
            game.nextLetter.focus();
        }
    }
}

export type WordleStatus = number;
export const INGAME: WordleStatus = 0;
export const WIN: WordleStatus = 1;
export const LOSE: WordleStatus = 2;

export type WordleColor = number;
export const GREY: WordleColor = 0;
export const YELLOW: WordleColor = 1;
export const GREEN: WordleColor = 2;

