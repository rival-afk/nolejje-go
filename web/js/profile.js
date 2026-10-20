const profileName = document.getElementById("profile-name");
const profileEmail = document.getElementById("profile-email");
const profileRole = document.getElementById("profile-role");
const avatarInput = document.getElementById("avatar-input");
const avatarPreview = document.getElementById("avatar-preview");
const avatarFileLabel = document.getElementById("avatar-file-label");
const saveNameBtn = document.getElementById("save-name");
const uploadAvatarBtn = document.getElementById("upload-avatar");
const removeAvatarBtn = document.getElementById("remove-avatar");
const deleteAccountBtn = document.getElementById("delete-account");
const profileMessage = document.getElementById("profile-message");

const roleLabels = {
  student: "Ученик",
  teacher: "Учитель",
  admin: "Администратор",
};

let profileData = null;
let pendingAvatarDataUrl = null;

function showMessage(text, danger = false) {
  profileMessage.textContent = text;
  profileMessage.classList.toggle("danger", danger);
  profileMessage.classList.toggle("success", !danger);
}

function getInitial(name) {
  return (name || "A").trim().charAt(0).toUpperCase() || "A";
}

function renderAvatar(avatarUrl, fallbackName) {
  if (avatarUrl) {
    avatarPreview.innerHTML = `<img src="${avatarUrl}" alt="Avatar">`;
    return;
  }

  avatarPreview.textContent = getInitial(fallbackName);
}

function applyProfile(user) {
  profileData = user;
  const roleLabel = roleLabels[user.role] || user.role || "Пользователь";

  profileName.value = user.name || "";
  profileEmail.value = user.email || "";
  profileRole.value = roleLabel;

  document.getElementById("header-username").textContent = user.name || "";
  document.getElementById("header-role-badge").textContent = roleLabel;
  document.getElementById("profile-status-role").textContent = roleLabel;
  document.getElementById("profile-status-email").textContent = user.email || "—";
  document.getElementById("profile-display-name").textContent = user.name || "Профиль";
  document.getElementById("profile-display-role").textContent = roleLabel;

  renderAvatar(user.avatar_url, user.name);
}

async function loadProfile() {
  try {
    const user = await checkAuth();
    if (!user) return;
    applyProfile(user);
  } catch (err) {
    showMessage("Не удалось загрузить профиль", true);
  }
}

async function updateName() {
  if (!profileData) {
    showMessage("Профиль ещё загружается", true);
    return;
  }

  const value = profileName.value.trim();
  if (!value) {
    showMessage("Имя не может быть пустым", true);
    return;
  }

  try {
    const token = localStorage.getItem("access_token");
    const response = await fetch("/api/users/me", {
      method: "PATCH",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + token
      },
      body: JSON.stringify({ name: value })
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      showMessage(error.detail || "Ошибка при сохранении имени", true);
      return;
    }

    const updatedUser = await response.json();
    applyProfile(updatedUser);
    showMessage("Имя сохранено");
  } catch (err) {
    showMessage("Сервер не отвечает", true);
  }
}

function readAvatarFile(file) {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(new Error("read_failed"));
    reader.readAsDataURL(file);
  });
}

async function onAvatarSelected() {
  const file = avatarInput.files[0];
  if (!file) {
    pendingAvatarDataUrl = null;
    avatarFileLabel.textContent = "Выбрать новый файл";
    renderAvatar(profileData?.avatar_url, profileData?.name);
    return;
  }

  if (file.size > 2 * 1024 * 1024) {
    avatarInput.value = "";
    pendingAvatarDataUrl = null;
    avatarFileLabel.textContent = "Выбрать новый файл";
    showMessage("Максимальный размер аватара — 2 МБ", true);
    return;
  }

  try {
    pendingAvatarDataUrl = await readAvatarFile(file);
    avatarFileLabel.textContent = file.name;
    renderAvatar(pendingAvatarDataUrl, profileData?.name);
    showMessage("Превью обновлено. Нажмите «Сохранить аватар».", false);
  } catch (err) {
    showMessage("Не удалось прочитать файл", true);
  }
}

async function uploadAvatar() {
  if (!profileData) {
    showMessage("Профиль ещё загружается", true);
    return;
  }

  if (!pendingAvatarDataUrl) {
    showMessage("Сначала выберите файл", true);
    return;
  }

  try {
    const token = localStorage.getItem("access_token");
    const response = await fetch("/api/users/me/avatar", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + token
      },
      body: JSON.stringify({ avatar_url: pendingAvatarDataUrl })
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      showMessage(error.detail || "Ошибка при загрузке аватара", true);
      return;
    }

    const data = await response.json();
    profileData.avatar_url = data.avatar_url || null;
    pendingAvatarDataUrl = null;
    avatarInput.value = "";
    avatarFileLabel.textContent = "Выбрать новый файл";
    renderAvatar(profileData.avatar_url, profileData.name);
    showMessage("Аватар обновлён");
  } catch (err) {
    showMessage("Ошибка сети", true);
  }
}

async function removeAvatar() {
  if (!profileData) {
    showMessage("Профиль ещё загружается", true);
    return;
  }

  try {
    const token = localStorage.getItem("access_token");
    const response = await fetch("/api/users/me/avatar", {
      method: "DELETE",
      headers: {
        "Authorization": "Bearer " + token
      }
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      showMessage(error.detail || "Ошибка удаления аватара", true);
      return;
    }

    pendingAvatarDataUrl = null;
    avatarInput.value = "";
    avatarFileLabel.textContent = "Выбрать новый файл";
    profileData.avatar_url = null;
    renderAvatar(null, profileData.name);
    showMessage("Аватар удалён");
  } catch (err) {
    showMessage("Ошибка сети", true);
  }
}

async function deleteAccount() {
  if (!profileData) {
    showMessage("Профиль ещё загружается", true);
    return;
  }

  if (!confirm("Вы уверены? Это действие удалит ваш аккаунт навсегда.")) {
    return;
  }

  try {
    const token = localStorage.getItem("access_token");
    const response = await fetch("/api/users/me", {
      method: "DELETE",
      headers: {
        "Authorization": "Bearer " + token
      }
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      showMessage(error.detail || "Ошибка удаления аккаунта", true);
      return;
    }

    localStorage.removeItem("access_token");
    window.location.href = "/login";
  } catch (err) {
    showMessage("Ошибка сети", true);
  }
}

window.addEventListener("DOMContentLoaded", async () => {
  await loadProfile();

  saveNameBtn.addEventListener("click", updateName);
  uploadAvatarBtn.addEventListener("click", uploadAvatar);
  removeAvatarBtn.addEventListener("click", removeAvatar);
  deleteAccountBtn.addEventListener("click", deleteAccount);
  avatarInput.addEventListener("change", onAvatarSelected);

  const logoutBtn = document.getElementById("logout-btn");
  if (logoutBtn) {
    logoutBtn.addEventListener("click", () => {
      localStorage.removeItem("access_token");
      window.location.href = "/login";
    });
  }
});
