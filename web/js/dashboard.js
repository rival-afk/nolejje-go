let user = null;

async function init() {
  user = await checkAuth();
  return user;
}

init().then(() => {
  const token = localStorage.getItem("access_token");
  const roleLabels = { student: "УЧЕНИК", teacher: "УЧИТЕЛЬ", admin: "АДМИН" };

  if (user.role !== "admin") {
    document.getElementById("header-username").textContent = user.name;
  }
  document.getElementById("header-role-badge").textContent = roleLabels[user.role] || user.role;

  function logout() {
    localStorage.removeItem("access_token");
    window.location.href = "/login";
  }

  const logoutBtn = document.getElementById("logout-btn");
  const logoutBtnAdmin = document.getElementById("logout-btn-admin");
  if (logoutBtn) logoutBtn.addEventListener("click", logout);
  if (logoutBtnAdmin) logoutBtnAdmin.addEventListener("click", logout);

  if (user.role === "student") {
    showDashboard("student");
    initStudent(token);
  } else if (user.role === "teacher") {
    showDashboard("teacher");
    initTeacher(token);
  } else if (user.role === "admin") {
    showDashboard("admin");
    initAdmin(token);
  } else {
    throw new Error("Неизвестная роль");
  }
});

function showDashboard(role) {
  document.getElementById("student-dashboard").style.display = "none";
  document.getElementById("teacher-dashboard").style.display = "none";
  document.getElementById("admin-dashboard").style.display   = "none";

  if (role === "student") document.getElementById("student-dashboard").style.display = "block";
  if (role === "teacher") document.getElementById("teacher-dashboard").style.display = "block";
  if (role === "admin")   document.getElementById("admin-dashboard").style.display   = "flex";
}

function setupTabs(containerSelector, onSwitch) {
  const tabs = document.querySelectorAll(containerSelector + " .dash-tab");
  tabs.forEach(tab => {
    tab.addEventListener("click", () => {
      tabs.forEach(t => t.classList.remove("active"));
      tab.classList.add("active");

      const targetId = "tab-" + tab.dataset.tab;
      document.querySelectorAll(".dash-tab-content").forEach(c => {
        c.classList.add("hidden");
      });
      const target = document.getElementById(targetId);
      if (target) target.classList.remove("hidden");

      if (onSwitch) onSwitch(tab.dataset.tab);
    });
  });
}

// ════════════════════════════════════════════════════════
// STUDENT
// ════════════════════════════════════════════════════════
async function initStudent(token) {
  document.getElementById("user-name").textContent = user.name.toUpperCase();

  const meData = await fetch("/api/students/me", {
    headers: { "Authorization": "Bearer " + token }
  }).then(r => r.json()).catch(() => ({}));

  if (meData.class) {
    document.getElementById("user-class").textContent = meData.class + " класс · " + meData.school_name;
  }
  if (meData.avg_grade != null) {
    document.getElementById("avg-grade").textContent = parseFloat(meData.avg_grade).toFixed(1);
  }

  setupTabs(".student-wrap", tab => {
    if (tab === "grades" && !document.getElementById("grades-list").dataset.loaded) {
      loadStudentGrades(token);
    }
    if (tab === "schedule" && !document.getElementById("schedule-list").dataset.loaded) {
      loadStudentSchedule(token);
    }
  });

  loadStudentHomework(token);
}

async function loadStudentHomework(token) {
  const homework = document.getElementById("homework");
  homework.innerHTML = "<p class='text'>Загрузка...</p>";

  const response = await fetch("/api/students/me/homeworks", {
    headers: { "Authorization": "Bearer " + token }
  });

  if (!response.ok) {
    homework.innerHTML = "<p class='empty-hw'>Ошибка загрузки заданий</p>";
    return;
  }

  const data = await response.json();
  document.getElementById("hw-count").textContent = data.length;
  homework.innerHTML = "";

  if (data.length === 0) {
    homework.innerHTML = "<p class='empty-hw'>Нет домашних заданий</p>";
    return;
  }

  data.forEach(hw => {
    const item     = document.createElement("div");
    const body     = document.createElement("div");
    const subjName = document.createElement("div");
    const title    = document.createElement("div");
    const desc     = document.createElement("div");
    const date     = document.createElement("div");

    item.classList.add("hw-card");
    body.classList.add("hw-card-body");
    subjName.classList.add("hw-card-subjname");
    title.classList.add("hw-card-title");
    desc.classList.add("hw-card-desc");
    date.classList.add("hw-card-date");

    subjName.textContent = hw.subject_name;
    title.textContent    = hw.title;
    desc.textContent     = hw.description;

    const d = new Date(hw.due_date);
    date.textContent = d.getDate() + "." + (d.getMonth() + 1) + "." + d.getFullYear();

    body.appendChild(subjName);
    body.appendChild(title);
    body.appendChild(desc);
    body.appendChild(date);
    item.appendChild(body);
    homework.appendChild(item);
  });
}

async function loadStudentGrades(token) {
  const container = document.getElementById("grades-list");
  container.dataset.loaded = "1";
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await fetch("/api/students/me/grades", {
      headers: { "Authorization": "Bearer " + token }
    }).then(r => r.json());

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='empty-hw'>Нет оценок</p>";
      return;
    }

    const bySubject = {};
    data.forEach(g => {
      if (!bySubject[g.subject_name]) bySubject[g.subject_name] = [];
      bySubject[g.subject_name].push(g);
    });

    Object.entries(bySubject).forEach(([subject, grades]) => {
      const avg = (grades.reduce((s, g) => s + g.value, 0) / grades.length).toFixed(1);

      const block = document.createElement("div");
      block.classList.add("grades-subject-block");

      const header = document.createElement("div");
      header.classList.add("grades-subject-header");
      header.innerHTML = `
        <span class="hw-card-subjname">${subject}</span>
        <span class="grades-avg">avg ${avg}</span>
      `;

      const pills = document.createElement("div");
      pills.classList.add("grades-pills");
      grades.forEach(g => {
        const pill = document.createElement("div");
        pill.classList.add("grade-pill");
        if (g.value >= 8) pill.classList.add("grade-high");
        else if (g.value >= 6) pill.classList.add("grade-mid");
        else pill.classList.add("grade-low");

        const d = new Date(g.date);
        pill.innerHTML = `
          <span class="grade-pill-value">${g.value}</span>
          <span class="grade-pill-date">${d.getDate()}.${d.getMonth() + 1}</span>
        `;
        pills.appendChild(pill);
      });

      block.appendChild(header);
      block.appendChild(pills);
      container.appendChild(block);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

async function loadStudentSchedule(token) {
  const container = document.getElementById("schedule-list");
  container.dataset.loaded = "1";
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  const dayNames = ["", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"];

  try {
    const data = await fetch("/api/students/me/schedule", {
      headers: { "Authorization": "Bearer " + token }
    }).then(r => r.json());

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='empty-hw'>Расписание не заполнено</p>";
      return;
    }

    const byDay = {};
    data.forEach(lesson => {
      if (!byDay[lesson.weekday]) byDay[lesson.weekday] = [];
      byDay[lesson.weekday].push(lesson);
    });

    Object.entries(byDay).sort(([a], [b]) => a - b).forEach(([day, lessons]) => {
      const block = document.createElement("div");
      block.classList.add("schedule-day");

      const dayLabel = document.createElement("div");
      dayLabel.classList.add("schedule-day-label");
      dayLabel.textContent = dayNames[day] || "День " + day;
      block.appendChild(dayLabel);

      lessons.forEach((lesson, i) => {
        const row = document.createElement("div");
        row.classList.add("schedule-row");
        row.innerHTML = `
          <span class="schedule-num">${i + 1}</span>
          <span class="schedule-subject">${lesson.subject_name}</span>
          <span class="schedule-teacher">${lesson.teacher_name}</span>
          ${lesson.room ? `<span class="schedule-room">каб. ${lesson.room}</span>` : ""}
        `;
        block.appendChild(row);
      });

      container.appendChild(block);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

// ════════════════════════════════════════════════════════
// TEACHER
// ════════════════════════════════════════════════════════
let activeClassId   = null;
let activeStudents  = [];

function initTeacher(token) {
  const teacherName = document.getElementById("teacher-name");
  if (teacherName) teacherName.textContent = user.name.toUpperCase();
  loadClasses(token);
}

function loadClasses(token) {
  fetch("/api/teacher/classes", { headers: { "Authorization": "Bearer " + token } })
    .then(r => r.json())
    .then(data => {
      const list = document.getElementById("class-list");
      list.innerHTML = "";

      if (data.length === 0) {
        list.innerHTML = "<span class='text'>Нет классов</span>";
        return;
      }

      data.forEach(cls => {
        const div = document.createElement("div");
        div.classList.add("class-item");
        div.innerHTML = `
          <div>${cls.name}</div>
          <div class="class-item-sub">${cls.students_count} учеников</div>
        `;
        div.addEventListener("click", () => {
          document.querySelectorAll(".class-item").forEach(el => el.classList.remove("active"));
          div.classList.add("active");
          activeClassId = cls.id;
          document.getElementById("class-title").textContent = cls.name;
          document.getElementById("teacher-tabs").style.display = "flex";

          document.querySelectorAll("#teacher-dashboard .dash-tab").forEach(t => {
            t.classList.remove("active");
            if (t.dataset.tab === "t-journal") t.classList.add("active");
          });
          document.querySelectorAll("#teacher-dashboard .dash-tab-content").forEach(c => c.classList.add("hidden"));
          document.getElementById("tab-t-journal").classList.remove("hidden");

          setupTeacherTabs(token, cls.id);
          loadJournal(token, cls.id);
        });
        list.appendChild(div);
      });
    })
    .catch(() => {});
}

function setupTeacherTabs(token, classId) {
  const tabs = document.querySelectorAll("#teacher-dashboard .dash-tab");
  tabs.forEach(tab => {
    const fresh = tab.cloneNode(true);
    tab.parentNode.replaceChild(fresh, tab);
  });

  document.querySelectorAll("#teacher-dashboard .dash-tab").forEach(tab => {
    tab.addEventListener("click", () => {
      document.querySelectorAll("#teacher-dashboard .dash-tab").forEach(t => t.classList.remove("active"));
      tab.classList.add("active");

      document.querySelectorAll("#teacher-dashboard .dash-tab-content").forEach(c => c.classList.add("hidden"));
      const target = document.getElementById("tab-" + tab.dataset.tab);
      if (target) target.classList.remove("hidden");

      if (tab.dataset.tab === "t-journal") loadJournal(token, classId);
      if (tab.dataset.tab === "t-homework") loadStudents(classId, token);
      if (tab.dataset.tab === "t-schedule") loadTeacherSchedule(token);
    });
  });
}

async function loadJournal(token, classId) {
  const container = document.getElementById("grades-journal");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const [gradesData, studentsData] = await Promise.all([
      fetch("/api/teacher/classes/" + classId + "/grades", {
        headers: { "Authorization": "Bearer " + token }
      }).then(r => r.json()),
      fetch("/api/teacher/classes/" + classId + "/students", {
        headers: { "Authorization": "Bearer " + token }
      }).then(r => r.json())
    ]);

    activeStudents = studentsData;

    const gradeSelect = document.getElementById("grade-student-select");
    gradeSelect.innerHTML = "<option value=''>Ученик</option>";
    studentsData.forEach(s => {
      const opt = document.createElement("option");
      opt.value = s.id;
      opt.textContent = s.name;
      gradeSelect.appendChild(opt);
    });

    const dateInput = document.getElementById("grade-date");
    if (!dateInput.value) {
      dateInput.value = new Date().toISOString().split("T")[0];
    }

    document.getElementById("grade-form").style.display = "block";
    initGradeButton(token, classId);

    container.innerHTML = "";

    if (gradesData.length === 0) {
      container.innerHTML = "<p class='empty-hw'>Нет оценок</p>";
      return;
    }

    const byStudent = {};
    gradesData.forEach(g => {
      if (!byStudent[g.student_name]) byStudent[g.student_name] = [];
      byStudent[g.student_name].push(g);
    });

    Object.entries(byStudent).forEach(([studentName, grades]) => {
      const block = document.createElement("div");
      block.classList.add("journal-row");

      const name = document.createElement("div");
      name.classList.add("journal-student-name");
      name.textContent = studentName;

      const pills = document.createElement("div");
      pills.classList.add("grades-pills");

      grades.forEach(g => {
        const pill = document.createElement("div");
        pill.classList.add("grade-pill", "grade-pill-deletable");
        if (g.value >= 8) pill.classList.add("grade-high");
        else if (g.value >= 6) pill.classList.add("grade-mid");
        else pill.classList.add("grade-low");

        const d = new Date(g.date);
        pill.innerHTML = `
          <span class="grade-pill-value">${g.value}</span>
          <span class="grade-pill-date">${d.getDate()}.${d.getMonth() + 1}</span>
        `;

        pill.addEventListener("click", async () => {
          pill.style.opacity = "0.4";
          await fetch("/api/teacher/grades/" + g.grade_id, {
            method: "DELETE",
            headers: { "Authorization": "Bearer " + token }
          });
          loadJournal(token, classId);
        });

        pills.appendChild(pill);
      });

      block.appendChild(name);
      block.appendChild(pills);
      container.appendChild(block);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

function initGradeButton(token, classId) {
  const btn = document.getElementById("add-grade-btn");
  const fresh = btn.cloneNode(true);
  btn.parentNode.replaceChild(fresh, btn);

  fresh.addEventListener("click", async () => {
    const student_id = document.getElementById("grade-student-select").value;
    const value      = parseInt(document.getElementById("grade-value").value);
    const date       = document.getElementById("grade-date").value;
    const errorEl    = document.getElementById("grade-error");

    if (!student_id || !value || !date) {
      errorEl.textContent = "Заполни все поля";
      errorEl.style.display = "block";
      return;
    }

    if (value < 1 || value > 10) {
      errorEl.textContent = "Балл от 1 до 10";
      errorEl.style.display = "block";
      return;
    }

    errorEl.style.display = "none";

    await fetch("/api/teacher/grades", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + token
      },
      body: JSON.stringify({ student_id: parseInt(student_id), value, date })
    });

    document.getElementById("grade-value").value = "";
    loadJournal(token, classId);
  });
}

function loadStudents(classId, token) {
  const container = document.getElementById("students");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  fetch("/api/teacher/classes/" + classId + "/students", {
    headers: { "Authorization": "Bearer " + token }
  })
    .then(r => r.json())
    .then(data => {
      container.innerHTML = "";

      if (data.length === 0) {
        container.innerHTML = "<p class='text'>Нет учеников</p>";
        return;
      }

      data.forEach(student => {
        const row = document.createElement("div");
        row.classList.add("student-row");
        row.innerHTML = `<span class="student-row-name">${student.name}</span>`;
        container.appendChild(row);
      });
    })
    .catch(() => {
      container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
    });
}

async function loadTeacherSchedule(token) {
  const container = document.getElementById("teacher-schedule");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  const dayNames = ["", "Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"];

  try {
    const data = await fetch("/api/teacher/schedule", {
      headers: { "Authorization": "Bearer " + token }
    }).then(r => r.json());

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='empty-hw'>Расписание не заполнено</p>";
      return;
    }

    const byDay = {};
    data.forEach(lesson => {
      if (!byDay[lesson.weekday]) byDay[lesson.weekday] = [];
      byDay[lesson.weekday].push(lesson);
    });

    Object.entries(byDay).sort(([a], [b]) => a - b).forEach(([day, lessons]) => {
      const block = document.createElement("div");
      block.classList.add("schedule-day");

      const dayLabel = document.createElement("div");
      dayLabel.classList.add("schedule-day-label");
      dayLabel.textContent = dayNames[day] || "День " + day;
      block.appendChild(dayLabel);

      lessons.forEach((lesson, i) => {
        const row = document.createElement("div");
        row.classList.add("schedule-row");
        row.innerHTML = `
          <span class="schedule-num">${i + 1}</span>
          <span class="schedule-subject">${lesson.subject_name}</span>
          <span class="schedule-teacher">${lesson.class_name}</span>
          ${lesson.room ? `<span class="schedule-room">каб. ${lesson.room}</span>` : ""}
        `;
        block.appendChild(row);
      });

      container.appendChild(block);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

// ════════════════════════════════════════════════════════
// ADMIN
// ════════════════════════════════════════════════════════
function initAdmin(token) {
  const sidebar = document.getElementById("admin-username-sidebar");
  if (sidebar) sidebar.textContent = user.email || "";

  setupAdminNav(token);
  loadPage("assign", token);
}

function setupAdminNav(token) {
  document.querySelectorAll(".nav-item").forEach(item => {
    item.addEventListener("click", () => {
      document.querySelectorAll(".nav-item").forEach(i => i.classList.remove("active"));
      item.classList.add("active");
      loadPage(item.dataset.page, token);
    });
  });
}

async function requestJson(url, options = {}) {
  const response = await fetch(url, options);
  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    throw new Error(data.detail || "Ошибка запроса");
  }

  return data;
}

function loadPage(page, token) {
  const container = document.getElementById("admin-content");

  if (page === "assign") {
    container.innerHTML = `
      <h1 class="admin-page-title">Назначения</h1>
      <div id="assign-panel"></div>
      <div class="assignments-list" id="assignments"></div>
    `;
    initAssignPage(token);
    return;
  }

  if (page === "teachers") {
    container.innerHTML = `
      <div class="admin-page-head">
        <h1 class="admin-page-title">Учителя</h1>
        <p class="admin-page-subtitle">Список преподавателей, которых можно назначать на классы и предметы.</p>
      </div>
      <div class="assignments-list" id="teachers-list"></div>
    `;
    loadTeachers(token);
    return;
  }

  if (page === "classes") {
    container.innerHTML = `
      <div class="admin-page-head">
        <h1 class="admin-page-title">Классы</h1>
        <p class="admin-page-subtitle">Создание новых классов и просмотр текущей структуры по школе.</p>
      </div>
      <div class="admin-form-card">
        <div class="admin-form-grid admin-form-grid-3">
          <input class="dash-input" id="class-number" type="number" min="1" placeholder="Номер">
          <input class="dash-input" id="class-letter" type="text" maxlength="2" placeholder="Буква">
          <input class="dash-input" id="class-school" type="text" placeholder="Школа">
        </div>
        <div class="admin-form-actions">
          <button class="accent-button" id="create-class-btn">Создать класс</button>
        </div>
        <div class="assign-error" id="class-error"></div>
      </div>
      <div class="assignments-list" id="classes-list"></div>
    `;
    initClassCreateForm(token);
    loadAdminClasses(token);
    return;
  }

  if (page === "subjects") {
    container.innerHTML = `
      <div class="admin-page-head">
        <h1 class="admin-page-title">Предметы</h1>
        <p class="admin-page-subtitle">Добавление предметов к классам до назначения учителей.</p>
      </div>
      <div class="admin-form-card">
        <div class="admin-form-grid admin-form-grid-2">
          <input class="dash-input" id="subject-name" type="text" placeholder="Название предмета">
          <select class="assign-select" id="subject-class-select"></select>
        </div>
        <div class="admin-form-actions">
          <button class="accent-button" id="create-subject-btn">Создать предмет</button>
        </div>
        <div class="assign-error" id="subject-error"></div>
      </div>
      <div class="assignments-list" id="subjects-list"></div>
    `;
    initSubjectCreateForm(token);
    loadSubjects(token);
    return;
  }

  if (page === "admins") {
    container.innerHTML = `
      <div class="admin-page-head">
        <h1 class="admin-page-title">Админы</h1>
        <p class="admin-page-subtitle">Создание новых административных аккаунтов без открытия публичной регистрации.</p>
      </div>
      <div class="admin-form-card">
        <div class="admin-form-grid admin-form-grid-3">
          <input class="dash-input" id="admin-name" type="text" placeholder="Имя">
          <input class="dash-input" id="admin-email" type="email" placeholder="Почта">
          <input class="dash-input" id="admin-password" type="password" placeholder="Пароль">
        </div>
        <div class="admin-form-actions">
          <button class="accent-button" id="create-admin-btn">Создать админа</button>
        </div>
        <div class="assign-error" id="admin-error"></div>
      </div>
      <div class="assignments-list" id="admins-list"></div>
    `;
    initAdminCreateForm(token);
    loadAdmins(token);
    return;
  }

  if (page === "analytics") {
    container.innerHTML = `
      <div class="admin-page-head">
        <h1 class="admin-page-title">Аналитика</h1>
        <p class="admin-page-subtitle">Сводные показатели пользователей, ролей, классов и среднего балла.</p>
      </div>
      <div id="analytics-content"></div>
    `;
    loadAnalytics(token);
    return;
  }

  if (page === "reports") {
    container.innerHTML = `
      <h1 class="admin-page-title">Жалобы</h1>
      <p class="text">Нет жалоб</p>
    `;
    return;
  }
}

function initAssignPage(token) {
  renderAssignUI(token);
  loadAssignments(token);
}

function renderAssignUI(token) {
  const container = document.getElementById("assign-panel");
  container.innerHTML = `
    <div class="assign-form">
      <select class="assign-select" id="teacher-select"></select>
      <select class="assign-select" id="class-select"></select>
      <select class="assign-select" id="subject-select"></select>
      <button class="accent-button" id="assign-btn">Назначить</button>
    </div>
    <div class="assign-error" id="assign-error"></div>
  `;
  loadAssignData(token);
}

async function loadAssignData(token) {
  const errorEl = document.getElementById("assign-error");
  try {
    const [teachers, classes, subjects] = await Promise.all([
      requestJson("/api/admin/teachers", { headers: { Authorization: "Bearer " + token } }),
      requestJson("/api/classes"),
      requestJson("/api/admin/subjects", { headers: { Authorization: "Bearer " + token } })
    ]);

    fillSelect("teacher-select", teachers, "Учитель");
    fillSelect("class-select",   classes.map(item => ({ id: item.id, name: item.class })), "Класс");
    fillSelect("subject-select", subjects, "Предмет");

    initAssignButton(token);
  } catch {
    errorEl.textContent = "Ошибка загрузки данных";
    errorEl.style.display = "block";
  }
}

function fillSelect(id, data, placeholder) {
  const select = document.getElementById(id);
  select.innerHTML = `<option value="">${placeholder}</option>`;
  data.forEach(item => {
    const option = document.createElement("option");
    option.value = item.id;
    option.textContent = item.name;
    select.appendChild(option);
  });
}

function initAssignButton(token) {
  const btn     = document.getElementById("assign-btn");
  const errorEl = document.getElementById("assign-error");

  btn.addEventListener("click", async () => {
    const teacher_id = document.getElementById("teacher-select").value;
    const class_id   = document.getElementById("class-select").value;
    const subject_id = document.getElementById("subject-select").value;

    if (!teacher_id || !class_id || !subject_id) {
      errorEl.textContent = "Заполни все поля";
      errorEl.style.display = "block";
      return;
    }

    errorEl.style.display = "none";

    try {
      await fetch("/api/admin/assign", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": "Bearer " + token
        },
        body: JSON.stringify({ teacher_id, class_id, subject_id })
      });

      loadAssignments(token);
      btn.textContent = "✓ Готово";
      setTimeout(() => btn.textContent = "Назначить", 1200);
    } catch {
      errorEl.textContent = "Ошибка назначения";
      errorEl.style.display = "block";
    }
  });
}

async function loadAssignments(token) {
  const container = document.getElementById("assignments");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await fetch("/api/admin/assignments", {
      headers: { "Authorization": "Bearer " + token }
    }).then(r => r.json());

    renderAssignments(data, token);
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

function renderAssignments(data, token) {
  const container = document.getElementById("assignments");
  container.innerHTML = "";

  if (data.length === 0) {
    container.innerHTML = "<p class='list-empty'>Нет назначений</p>";
    return;
  }

  data.forEach(item => {
    const div = document.createElement("div");
    div.classList.add("assignment-card");
    div.innerHTML = `
      <span class="assignment-main">
        <span class="assignment-teacher">${item.teacher}</span>
        <span class="assignment-arrow"> → </span>
        <span class="assignment-class">${item.class}</span>
        <span class="assignment-arrow"> → </span>
        <span class="assignment-subject">${item.subject}</span>
      </span>
      <button class="delete-btn" data-id="${item.id}">✕</button>
    `;

    div.querySelector(".delete-btn").addEventListener("click", async () => {
      await fetch("/api/admin/assign/" + item.id, {
        method: "DELETE",
        headers: { "Authorization": "Bearer " + token }
      });
      loadAssignments(token);
    });

    container.appendChild(div);
  });
}

async function loadTeachers(token) {
  const container = document.getElementById("teachers-list");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await requestJson("/api/admin/teachers", {
      headers: { "Authorization": "Bearer " + token }
    });

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='list-empty'>Нет учителей</p>";
      return;
    }

    data.forEach(teacher => {
      const div = document.createElement("div");
      div.classList.add("assignment-card");
      div.innerHTML = `<span class="assignment-subject">${teacher.name}</span>`;
      container.appendChild(div);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

async function loadAdmins(token) {
  const container = document.getElementById("admins-list");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await requestJson("/api/admin/admins", {
      headers: { "Authorization": "Bearer " + token }
    });

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='list-empty'>Нет админов</p>";
      return;
    }

    data.forEach(admin => {
      const div = document.createElement("div");
      div.classList.add("assignment-card");
      div.innerHTML = `
        <span class="assignment-subject">${admin.name}</span>
        <span class="assignment-teacher">${admin.email}</span>
      `;
      container.appendChild(div);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

async function loadAdminClasses(token) {
  const container = document.getElementById("classes-list");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await requestJson("/api/admin/classes", {
      headers: { "Authorization": "Bearer " + token }
    });

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='list-empty'>Нет классов</p>";
      return;
    }

    data.forEach(cls => {
      const div = document.createElement("div");
      div.classList.add("assignment-card");
      div.innerHTML = `
        <span class="assignment-subject">${cls.name}</span>
        <span class="assignment-teacher">${cls.students_count} учеников</span>
      `;
      container.appendChild(div);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

async function loadSubjects(token) {
  const container = document.getElementById("subjects-list");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await requestJson("/api/admin/subjects", {
      headers: { "Authorization": "Bearer " + token }
    });

    container.innerHTML = "";

    if (data.length === 0) {
      container.innerHTML = "<p class='list-empty'>Нет предметов</p>";
      return;
    }

    data.forEach(subject => {
      const div = document.createElement("div");
      div.classList.add("assignment-card");
      div.innerHTML = `
        <span class="assignment-subject">${subject.name}</span>
        <span class="assignment-teacher">${subject.class}</span>
      `;
      container.appendChild(div);
    });
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}

async function initClassCreateForm(token) {
  const btn = document.getElementById("create-class-btn");
  const errorEl = document.getElementById("class-error");

  btn.addEventListener("click", async () => {
    const number = Number(document.getElementById("class-number").value);
    const letter = document.getElementById("class-letter").value.trim();
    const school_name = document.getElementById("class-school").value.trim();

    if (!number || !letter || !school_name) {
      errorEl.textContent = "Заполни номер, букву и школу";
      errorEl.style.display = "block";
      return;
    }

    try {
      await requestJson("/api/admin/classes", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": "Bearer " + token
        },
        body: JSON.stringify({ number, letter, school_name })
      });

      errorEl.style.display = "none";
      document.getElementById("class-number").value = "";
      document.getElementById("class-letter").value = "";
      document.getElementById("class-school").value = "";
      loadAdminClasses(token);
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.style.display = "block";
    }
  });
}

async function initSubjectCreateForm(token) {
  const errorEl = document.getElementById("subject-error");
  const classSelect = document.getElementById("subject-class-select");
  classSelect.innerHTML = "<option value=''>Класс</option>";

  try {
    const classes = await requestJson("/api/classes");
    classes.forEach(item => {
      const option = document.createElement("option");
      option.value = item.id;
      option.textContent = item.class;
      classSelect.appendChild(option);
    });
  } catch (error) {
    errorEl.textContent = "Не удалось загрузить классы";
    errorEl.style.display = "block";
  }

  document.getElementById("create-subject-btn").addEventListener("click", async () => {
    const name = document.getElementById("subject-name").value.trim();
    const class_id = Number(classSelect.value);

    if (!name || !class_id) {
      errorEl.textContent = "Укажи предмет и класс";
      errorEl.style.display = "block";
      return;
    }

    try {
      await requestJson("/api/admin/subjects", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": "Bearer " + token
        },
        body: JSON.stringify({ name, class_id })
      });

      errorEl.style.display = "none";
      document.getElementById("subject-name").value = "";
      classSelect.value = "";
      loadSubjects(token);
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.style.display = "block";
    }
  });
}

function initAdminCreateForm(token) {
  const btn = document.getElementById("create-admin-btn");
  const errorEl = document.getElementById("admin-error");

  btn.addEventListener("click", async () => {
    const name = document.getElementById("admin-name").value.trim();
    const email = document.getElementById("admin-email").value.trim();
    const password = document.getElementById("admin-password").value;

    if (!name || !email || !password) {
      errorEl.textContent = "Заполни имя, почту и пароль";
      errorEl.style.display = "block";
      return;
    }

    try {
      await requestJson("/api/admin/users", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Authorization": "Bearer " + token
        },
        body: JSON.stringify({ name, email, password })
      });

      errorEl.style.display = "none";
      document.getElementById("admin-name").value = "";
      document.getElementById("admin-email").value = "";
      document.getElementById("admin-password").value = "";
      loadAdmins(token);
    } catch (error) {
      errorEl.textContent = error.message;
      errorEl.style.display = "block";
    }
  });
}

async function loadAnalytics(token) {
  const container = document.getElementById("analytics-content");
  container.innerHTML = "<p class='text'>Загрузка...</p>";

  try {
    const data = await fetch("/api/admin/analytics", {
      headers: { "Authorization": "Bearer " + token }
    }).then(r => r.json());

    const a   = data[0];
    const avg = a.avg_grade ? parseFloat(a.avg_grade).toFixed(1) : "—";

    container.innerHTML = `
      <div class="analytics-grid">
        <div class="stat-card">
          <span class="stat-label">Пользователей</span>
          <span class="stat-value">${a.total_users}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Учеников</span>
          <span class="stat-value">${a.total_students}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Учителей</span>
          <span class="stat-value">${a.total_teachers}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Классов</span>
          <span class="stat-value">${a.total_classes}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Средний балл</span>
          <span class="stat-value">${avg}</span>
        </div>
        <div class="stat-card">
          <span class="stat-label">Админов</span>
          <span class="stat-value">${a.total_admins}</span>
        </div>
      </div>
    `;
  } catch {
    container.innerHTML = "<p class='text'>Ошибка загрузки</p>";
  }
}
