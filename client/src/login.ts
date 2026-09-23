import { Popup } from "./popup.js";

export { login, logout };

const loginURL = "/login"
const signupURL = "/signup"
const tokenURL = "/token"

export type UserData = {
    validUser: boolean,
    id: number,
    jwt: string,
    username: string,
}

export type SignupRes = {
    usernameAvailable: boolean,
}

async function login(): Promise<UserData> {
    const jwt = window.localStorage.getItem("jwt");
    if (jwt != null) {
        const options = {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(jwt),
        };
        const userData: UserData = await fetch(tokenURL, options)
        .then(response => {
            if (!response.ok) {
                throw new Error(`Error posting jwt. Status: ${response.status}`);
            }
            return response.json();
        })
        .then(responseData => {
            return responseData;
        })
        .catch(error => {
            console.error('Error parsing login response json(jwt check):', error);
            const userData: UserData = {
                validUser: false,
                id: -999,
                jwt: "",
                username: ""
            };
            return userData;
        });

        if (userData.validUser) {
            return userData;
        }
    }

    const popup = Popup.open("tpl-login", null, false)!;
    popup.on(popup.q("#openSignup"), "click", () => openSignup(popup));
    const userData = await submit(popup);
    popup.close();
    return userData;
}

function logout() {
    window.localStorage.removeItem("jwt");
    window.location.reload();
}

async function postJSON(url: string, data: unknown): Promise<any> {
    const response = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
    if (!response.ok) {
        throw new Error(await response.text() || `Status ${response.status}`);
    }
    return response.json();
}

// Resolves once the player logs in successfully
function submit(popup: Popup): Promise<UserData> {
    return new Promise((resolve) => {
        const errtxt = popup.q<HTMLParagraphElement>("#errorText");
        const attempt = async () => {
            const data = {
                username: popup.q<HTMLInputElement>("#uname").value,
                password: popup.q<HTMLInputElement>("#psw").value,
            };
            try {
                const userData: UserData = await postJSON(loginURL, data);
                if (userData.validUser) {
                    window.localStorage.setItem("jwt", userData.jwt);
                    resolve(userData);
                } else {
                    errtxt.innerText = "Invalid username/password.";
                }
            } catch (error) {
                console.error('Error logging in:', error);
                errtxt.innerText = "Couldn't reach the server, try again.";
            }
        };
        popup.on(popup.q("#submitLogin"), "click", attempt);
        popup.on(popup.q("#psw"), "keydown", (e) => {
            if ((e as KeyboardEvent).key === "Enter") {
                attempt();
            }
        });
    });
}

// The signup form, shown on top of the login form
function openSignup(popup: Popup) {
    const layer = popup.addLayer("tpl-signup");
    const errDiv = layer.querySelector("#error") as HTMLDivElement;
    const close = () => popup.removeLayer(layer);

    layer.querySelector("#deleteSignup")!.addEventListener("click", close);
    layer.querySelector("#submitSignup")!.addEventListener("click", async () => {
        const data = {
            username: (layer.querySelector("#signupuname") as HTMLInputElement).value,
            password: (layer.querySelector("#signuppsw") as HTMLInputElement).value,
        };
        try {
            const res: SignupRes = await postJSON(signupURL, data);
            if (res.usernameAvailable) {
                close();
            } else {
                errDiv.textContent = "Username Taken";
                errDiv.style.display = "block";
            }
        } catch (error) {
            // The server explains what was wrong, e.g. the username is too long
            errDiv.textContent = error instanceof Error ? error.message : "Signup failed";
            errDiv.style.display = "block";
        }
    });
}
