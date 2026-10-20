async function checkAuth() {
  try {
    const token = await refreshToken();

    if (!token) {
      window.location.href = "/login";
      return;
    };

    const response = await fetch("/api/users/me", {
      method: "GET",
      headers: {
        "Authorization": "Bearer " + token
      }
    });

    if (response.status == 401) {
      window.location.href = "/login";
      return;
    };

    if (!response.ok) {
      throw new Error("Ошибка запроса");
    };

    const user = await response.json();
    return user;
  } catch (e) {
    throw new Error(e);
  };
};

function parseToken(token) {
  const payload = JSON.parse(atob(token.split(".")[1]));
  return payload;
};

async function refreshToken() {
  const token = localStorage.getItem("access_token");
  const timeLeft = parseToken(token);
  const now = Date.now() / 1000;
  let data = null;

  if (!(timeLeft.exp - now < 300)) {
    return token;
  };

  const response = await fetch("/api/auth/refresh", {
    method: "GET",
    headers: {
      "Authorization": "Bearer " + token
    }
  });

  if (response.status == 401) {
    data = await response.json().catch((e) => {
      throw new Error(e);
    });

    if (data == null) {
      throw new Error("data is null");
    };

    if (data.detail == "Expired Token") {
      window.location.href = "/login";
      throw new Error("Токен истек. Редирект...")
    }
  }

  if (!response.ok) {
    throw new Error("Ошибка при получении токена. Вероятно токен не действителен");
  };

  localStorage.setItem("access_token", data.access_token);
  return data.access_token;
}