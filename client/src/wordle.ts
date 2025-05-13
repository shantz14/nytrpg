const GUESSES = 5;

export class Wordle {
    word: string;
    submitButton: HTMLButtonElement | null;
    nextLetter: HTMLInputElement | null;
    currentGuess: number;
    
    constructor() {
        this.word = this.getWord();
        this.submitButton = null;
        this.nextLetter = null;
        this.currentGuess = 0;
    }

    public run() {
        this.displayGame();

        this.populateGame();    
    }

    // TODO word load from database!!!
    private getWord(): string {

        return "GAMER";
    }

    private displayGame() {
        const gameHtml = `
        <button id="exit">X</button>

        <div class="gameContainer" id="gameContainer">

            <div class="wordContainer" id="wordContainer0">
                <input type="text" class="letter" id="letter00" 
                maxlength="1" autocomplete="off">
            </div>

        </div>

        <button id="submit">Submit</button>
        `;
        const popup = document.createElement("div");
        popup.setAttribute("id", "wordlePopup")
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

            for (let i = 0; i < this.word.length; i++) {
                let box = document.getElementById(baseId + i) as HTMLInputElement;
                boxes.push(box);


                if (box) {
                    const letter = box.value;
                    guess = guess + letter;
                }
            }

            if (guess.length == this.word.length) {
                this.colorMyBoxes(boxes, guess);
                this.currentGuess++;
                
                if (guess === this.word) {
                    this.win();
                } else if (this.currentGuess == GUESSES) {
                    this.lose();
                }

                if (this.nextLetter) {
                    this.nextLetter.focus();
                } else {
                    console.error("Could not find letter to focus on.");
                }
            }
        });

        const exitButton = document.getElementById("exit") as HTMLButtonElement;
        exitButton.addEventListener("click", () => {
            popup.remove();
        });

        document.addEventListener("keypress", (e: KeyboardEvent) => {
            if (e.key === "Enter") {
                e.preventDefault();
                this.submitButton?.click();
            }
        })

    }

    private deleteGame() {
        const popup = document.getElementById("wordlePopup") as HTMLDivElement;
        popup.remove();
    }

    private win() {
        console.log("YOU WIN");

        this.deleteGame();
    }

    private lose() {
        console.log("YOU LOSE");

        this.deleteGame();
    }

    private colorMyBoxes(boxes: Array<HTMLInputElement>, guess: string) {
        const letterCounts = this.countLetters();

        const lettersCounted: {[key: string]: number} = {};
        for (let i = 0; i < guess.length; i++) {
            lettersCounted[guess[i]] = 0;
        }
        
        //greys
        for (let i = 0; i < guess.length; i++) {
            boxes[i].style.backgroundColor = "grey";
        }
        //greens
        for (let i = 0; i < guess.length; i++) {
            if (guess[i] == this.word[i]) {
                boxes[i].style.backgroundColor = "green";
                lettersCounted[guess[i]]++;
            }
        }
        //yellows
        for (let i = 0; i < guess.length; i++) {
            if (lettersCounted[guess[i]] < letterCounts[guess[i]] && this.word.includes(guess[i]) && boxes[i].style.backgroundColor === "grey") {
                boxes[i].style.backgroundColor = "yellow";
                lettersCounted[guess[i]]++;
            }
        }
    }

    private countLetters() {
        let letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
        let letterCounts: {[key: string]: number} = {};
        for (let i = 0; i < letters.length; i++) {
            letterCounts[letters[i]] = 0;
        }
        for (let i = 0; i < this.word.length; i++) {
            letterCounts[this.word[i]]++;
        }
        return letterCounts;
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
        this.populateWord(wordContainer)

        document.querySelector("input")?.remove();

        const letter0 = document.getElementById("letter00");
        if (letter0) {
            letter0.focus();
        }
    }

    private populateWord(wordContainer: HTMLDivElement) {
        const letterDiv = document.getElementById("letter00") as HTMLInputElement;
        for (let c = 0; c < this.word.length; c++) {
            let newLetter = letterDiv.cloneNode(true) as HTMLInputElement;
            newLetter.id = "letter" + wordContainer.id[wordContainer.id.length - 1] + c;

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

        if (col == game.word.length - 1) {
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
