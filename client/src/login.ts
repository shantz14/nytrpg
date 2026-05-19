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

    createLoginPopup();
    handleSignup();
    const userData = await submit();
    deleteLoginPopup();
    return userData;
}

function logout() {
    window.localStorage.removeItem("jwt");
    window.location.reload();
}

async function submit(): Promise<UserData> {
    const submitButton = document.getElementById("submitLogin") as HTMLButtonElement;
    return new Promise((resolve, reject) => {
        var listener = () => {
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
                if (!response.ok) {
                    throw new Error(`Error posting login data. Status: ${response.status}`);
                }
                return response.json();
            })
            .then(responseData => {
                userData = responseData;
                if (userData.validUser) {
                    submitButton?.removeEventListener("click", listener);
                    window.localStorage.setItem("jwt", userData.jwt);
                    resolve(userData);
                } else {
                    const errtxt = document.getElementById("errorText") as HTMLParagraphElement;
                    errtxt.innerText = "Invalid username/password.";
                }
            })
            .catch(error => {
                console.error('Error parsing login response json:', error);
                reject();
            });
        };
        submitButton?.addEventListener("click", listener);
    });
}

function createLoginPopup() {
    const tpl = document.getElementById("tpl-login") as HTMLTemplateElement;
    document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));
}

function handleSignup() {
    const openSignup = document.getElementById("openSignup") as HTMLButtonElement;
    openSignup.addEventListener("click", createSignupPopup);
}

function createSignupPopup() {
    const tpl = document.getElementById("tpl-signup") as HTMLTemplateElement;
    document.getElementById("container")!.appendChild(tpl.content.cloneNode(true));

    const deleteSignup = document.getElementById("deleteSignup") as HTMLButtonElement;
    deleteSignup?.addEventListener("click", deleteSignupPopup);

    const signup = document.getElementById("submitSignup") as HTMLButtonElement;
    signup.addEventListener("click", submitSignup);
}

function submitSignup() {
    const unameEl = document.getElementById("signupuname") as HTMLInputElement;
    const pswEl = document.getElementById("signuppsw") as HTMLInputElement;
    const submitButton = document.getElementById("submitSignup") as HTMLButtonElement;

    const data = {
        username: unameEl.value,
        password: pswEl.value,
    };
    let res: SignupRes;
    const options = {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
    };
    fetch(signupURL, options)
    .then(response => {
        if (!response.ok) {
            throw new Error(`Error posting signup data. Status: ${response.status}`);
        }
        return response.json();
    })
    .then(responseData => {
        res = responseData;
        if (res.usernameAvailable) {
            deleteSignupPopup();
        } else {
            const errDiv = document.getElementById("error") as HTMLDivElement;
            errDiv.innerHTML = "Username Taken";
            errDiv.style.display = "block";
       }
    })
    .catch(error => {
        console.error('Error parsing login response json:', error);
        return;
    });
}

function deleteSignupPopup() {
    const popup = document.getElementById("signupPopup") as HTMLDivElement;
    popup.remove();
}

function deleteLoginPopup() {
    const popup = document.getElementById("loginPopup") as HTMLDivElement;
    popup.remove();
}

