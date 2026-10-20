const loginTab = document.getElementById("loginTab");
const registerTab = document.getElementById("registerTab");
const login = document.getElementById("login");
const register = document.getElementById("register");

const registerName = document.getElementById("register-name");
const registerEmail = document.getElementById("register-email");
const registerPassword = document.getElementById("register-password");
const classesList = document.getElementById("class-select");
const loginEmail = document.getElementById("login-email");
const loginPassword = document.getElementById("login-password");
const loginButton = document.getElementById("login-button");
const registerButton = document.getElementById("register-button");

const roleStudentButton = document.getElementById("student");
const roleTeacherButton = document.getElementById("teacher");

let role = null;
let classId = null;

function showError(message) {
    const errorDiv = document.getElementById("error");
    errorDiv.textContent = message;
    errorDiv.style.display = "block";
    setTimeout(() => {
        errorDiv.style.display = "none";
    }, 5000);
}

function changeContainerSize() {
    const container = document.querySelector("#container");
    const visibleForm = document.querySelector(".form.visible");
    if (visibleForm) {
        const height = visibleForm.offsetHeight;
        container.style.minHeight = height + "px";
    }
}

function setActiveRole(activeButton) {
    [roleStudentButton, roleTeacherButton].forEach(btn => {
        btn.classList.remove("selected");
    });
    activeButton.classList.add("selected");
}

roleStudentButton.addEventListener("click", function () {
    setActiveRole(roleStudentButton);
    role = 'student';
    classesList.classList.remove("hidden");
});
roleTeacherButton.addEventListener("click", function () {
    setActiveRole(roleTeacherButton);
    role = 'teacher';
    classesList.classList.add("hidden");
    classId = null;
});

classesList.addEventListener("change", function () {
    classId = classesList.value ? Number(classesList.value) : null;
});

fetch("/api/classes")
    .then(response => response.json())
    .then(data => {
        data.forEach(item => {
            const option = document.createElement("option");
            option.value = item.id;
            option.textContent = item.name + " " + item.class;
            classesList.appendChild(option);
        });
    })
    .catch(() => {});

loginTab.addEventListener("click", function () {
    login.classList.add("visible");
    login.classList.remove("hidden");
    register.classList.add("hidden");
    register.classList.remove("visible");
    loginTab.classList.add("active");
    registerTab.classList.remove("active");
    const bg = document.querySelector(".tab-bg");
    bg.style.transform = "translateX(0%)";
    changeContainerSize();
});

registerTab.addEventListener("click", function () {
    login.classList.add("hidden");
    login.classList.remove("visible");
    register.classList.add("visible");
    register.classList.remove("hidden");
    loginTab.classList.remove("active");
    registerTab.classList.add("active");
    const bg = document.querySelector(".tab-bg");
    bg.style.transform = "translateX(100%)";
    changeContainerSize();
});

loginButton.addEventListener("click", function () {
    const email = loginEmail.value.trim();
    const password = loginPassword.value;
    fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: email, password: password })
    })
    .then(response => {
        if (!response.ok) {
            return response.json().then(err => { throw new Error(err.detail || "Ошибка входа"); });
        }
        return response.json();
    })
    .then(data => {
        localStorage.setItem("access_token", data.access_token);
        window.location.href = "/dashboard";
    })
    .catch(error => showError(error.message));
});

registerButton.addEventListener("click", async function () {
    const name = registerName.value.trim();
    const email = registerEmail.value.trim();
    const password = registerPassword.value;
    if (!name || !email || !password) {
        showError("Заполните все поля");
        return;
    }
    if (!role) {
        showError("Выберите роль");
        return;
    }
    if (role === "student" && !classId) {
        showError("Выберите класс");
        return;
    }
    try {
        const registerResponse = await fetch("/api/auth/register", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                name: name,
                email: email,
                password: password,
                role: role,
                class_id: classId
            })
        });
        if (!registerResponse.ok) {
            const errorData = await registerResponse.json();
            throw new Error(errorData.detail || "Ошибка регистрации");
        }
        const loginResponse = await fetch("/api/auth/login", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ email: email, password: password })
        });
        if (!loginResponse.ok) {
            throw new Error("Ошибка автоматического входа");
        }
        const loginData = await loginResponse.json();
        localStorage.setItem("access_token", loginData.access_token);
        const meResponse = await fetch("/api/users/me", {
            headers: { "Authorization": "Bearer " + loginData.access_token }
        });
        if (!meResponse.ok) {
            throw new Error("Не удалось получить профиль");
        }
        window.location.href = "/dashboard";
    } catch (err) {
        showError(err.message);
    }
});