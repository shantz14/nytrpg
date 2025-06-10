export { login };

function login() {
    createPopup();
}

function createPopup() {
    const html = `
    <div class="loginContainer" id="loginContainer">

        <label for="uname"><b>Username</b></label>
        <input type="text" placeholder="Enter Username" name="uname" required>

        <label for="psw"><b>Password</b></label>
        <input type="password" placeholder="Enter Password" name="psw" required>

        <button type="submit">Login</button>

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
