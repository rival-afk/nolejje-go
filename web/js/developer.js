const roleLabels = {
  student: "УЧЕНИК",
  teacher: "УЧИТЕЛЬ",
  admin: "АДМИН",
};

async function loadDeveloperInfo() {
  try {
    const response = await fetch("/api/developer/age", {
      headers: {
        "Authorization": "Bearer " + localStorage.getItem("access_token")
      }
    });

    if (!response.ok) {
      document.getElementById("developer-age").textContent = "Ошибка загрузки";
      document.getElementById("dev-age-span").textContent = "Ошибка загрузки";
      return;
    }

    const data = await response.json();
    const ageText = data.age ? `${data.age} лет` : "—";
    document.getElementById("developer-age").textContent = ageText;
    document.getElementById("dev-age-span").textContent = ageText;
  } catch (err) {
    document.getElementById("developer-age").textContent = "Ошибка";
    document.getElementById("dev-age-span").textContent = "Ошибка";
  }
}

window.addEventListener("DOMContentLoaded", async () => {
  const user = await checkAuth();
  if (user) {
    document.getElementById("header-username").textContent = user.name || "";
    document.getElementById("header-role-badge").textContent = roleLabels[user.role] || user.role || "";
  }

  await loadDeveloperInfo();

  const logoutBtn = document.getElementById("logout-btn");
  if (logoutBtn) {
    logoutBtn.addEventListener("click", () => {
      localStorage.removeItem("access_token");
      window.location.href = "/login";
    });
  }
});
