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

    constructor(userData: UserData) {
        this.userData = userData;
    }

    public run() {
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
        <div class="leaderboardContainer" id="leaderboardContainer">

        <h1>Wordle Rankings</h1>

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
    }

}

