async function initNavbar() {
  const token = localStorage.getItem("access_token");

  if (!token) return;

  try {
    const response = await fetch("/api/users/me", {
      headers: {
        Authorization: "Bearer " + token
      }
    });

    if (!response.ok) return;

    const user = await response.json();

    const authButtons = document.getElementById("auth-buttons");

    authButtons.innerHTML = `
      <a class="header-btn accent-btn" href="#">
        ${user.name}
      </a>
    `;
  } catch {}
}

initNavbar();