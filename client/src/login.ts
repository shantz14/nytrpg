export { login };

const loginURL = "http://localhost:8080/login"

export type UserData = {
    validUser: boolean,
    // other stuff
}

async function login(): Promise<UserData> {
    createPopup();
    const userData: UserData = await submit();
    deletePopup();
    return userData;
}

async function submit(): Promise<UserData> {
    const submitButton = document.getElementById("submitLogin") as HTMLButtonElement;
    return new Promise((resolve, reject) => {
        var listener = (event: Event) => {
            const unameEl = document.getElementById("uname") as HTMLInputElement;
            const pswEl = document.getElementById("psw") as HTMLInputElement;

            const data = {
                username: unameEl.value,
                password: pswEl.value,
            };
            let userData: UserData;
            const options = {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(data),
            };
            fetch(loginURL, options)
            .then(response => {
                console.log(response)
                if (!response.ok) {
                    throw new Error(`Error posting login data. Status: ${response.status}`);
                }
                return response.json();
            })
            .then(responseData => {
                userData = responseData;
                console.log(userData.validUser)
                submitButton?.removeEventListener("click", listener);
                resolve(userData);
            })
            .catch(error => {
                console.error('Error parsing login response json:', error);
                reject();
            });
        };
        submitButton?.addEventListener("click", listener);
    });
}

function createPopup() {
    const html = `
    <div class="loginContainer" id="loginContainer">

    <label for="uname"><b>Username</b></label>
        <input type="text" id="uname" placeholder="Enter Username" name="uname" required>

    <label for="psw"><b>Password</b></label>
        <input type="password" id="psw" placeholder="Enter Password" name="psw" required>

    <button id="submitLogin" type="submit">Login</button>

    </div>
    `;
    const popup = document.createElement("div");
    popup.setAttribute("id", "loginPopup")
    popup.innerHTML = html;

    const parent = document.getElementById("container");
    if (parent) {
        parent.appendChild(popup);
    } else {
        console.error("No parent to append popup to.");
    }
}

function deletePopup() {
    const popup = document.getElementById("loginPopup") as HTMLDivElement;
    popup.remove();
}

