import { InputDriver } from "./input-driver";
import { UserData } from "./login";

const URL = "/leaderboard";

type Row = {
    place: number,
    uname: string,
    guesses: number,
    time: number
}

export class Leaderboard {
    userData: UserData;
    inputDriver: InputDriver

    constructor(userData: UserData, inputDriver: InputDriver) {
        this.userData = userData;
        this.inputDriver = inputDriver;
    }

    public run() {
        this.inputDriver.gameFocused = false;
        this.createPopup();
        this.populate();
    }

    private async populate() {
        const table = document.getElementById("lb") as HTMLTableElement;

        const options = {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            },
        }
        const now = new Date(); // Get the current date and time
        now.setHours(now.getHours()-7);
        const year = now.getUTCFullYear();
        let month = (now.getUTCMonth() + 1).toString();
        if (Number(month) < 10) {
            month = '0' + month;
        }
        let day = now.getUTCDate().toString();
        if (Number(day) < 10) {
            day = '0' + day;
        }
        const date = year + '-' + month + '-' + day;
        const dateText = document.getElementById("date") as HTMLHeadingElement;
        dateText.innerHTML = date;
        const data: Array<Row> = await fetch(URL + `?date=${date}`, options)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error getting leadboard data. Status: ${response.status}`);
            }
            return response.json();
        })
        .then(responseData => {
            return responseData;
        })
        .catch(error => {
            console.error('Error parsing leaderboard data:', error);
        });

        for (const i in data) {
            const row = data[i];
            table.insertRow();
            const trs = document.querySelectorAll("tr");
            const newRow = trs[trs.length - 1];
            
            const place = document.createElement("td");
            place.innerHTML = row.place.toString();
            const uname = document.createElement("td");
            uname.innerHTML = row.uname.toString();
            const guesses = document.createElement("td");
            guesses.innerHTML = row.guesses.toString();
            const time = document.createElement("td");
            time.innerHTML = row.time.toString();

            newRow.append(place, uname, guesses, time);
        }
    }

    private createPopup() {
        const html = `
        <button id="exit">X</button>
        <div class="leaderboardContainer" id="leaderboardContainer">

        <h1>Wordle Rankings</h1>
        <h2 id="date"></h2>

        <table id="lb">
            <tr id="h">
                <th>Place</th>
                <th>Name</th>
                <th>Guesses</th>
                <th>Time</th>
            </tr>
            <tr>
            </tr>
        </table>

        <button id="pageUp">Pgup</button>
        <button id="pageDown">Pgdn</button>
        <div id="days">
            <button id="prevDay">Previous Day</button>
            <button id="nextDay">Next Day</button>
        </div>

        </div>
        `;
        const popup = document.createElement("div");
        popup.setAttribute("id", "leaderboardPopup")
        popup.innerHTML = html;

        const parent = document.getElementById("container");
        if (parent) {
            parent.appendChild(popup);
        } else {
            console.error("No parent to append popup to.");
        }

        const pgup = document.getElementById("pageUp");
        pgup?.addEventListener("click", this.pageUp);
        const pgdn = document.getElementById("pageDown");
        pgdn?.addEventListener("click", this.pageDown);
        const nextDay = document.getElementById("nextDay");
        nextDay?.addEventListener("click", this.nextDay);
        const prevDay = document.getElementById("prevDay");
        prevDay?.addEventListener("click", this.prevDay);
        const exit = document.getElementById("exit");
        exit?.addEventListener("click", () => {
            this.inputDriver.gameFocused = true;
            const popup = document.getElementById("leaderboardPopup") as HTMLDivElement;
            popup.remove();
        });
    }

    private nextDay() {

    }

    private prevDay() {

    }

    private pageUp() {

    }

    private pageDown() {

    }

}

