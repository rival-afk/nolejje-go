# Nolejje — Электронный дневник

[![Python](https://img.shields.io/badge/Python-3.10%2B-blue)](https://python.org) [![FastAPI](https://img.shields.io/badge/FastAPI-0.95%2B-green)](https://fastapi.tiangolo.com) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14%2B-blue)](https://www.postgresql.org)  

**Nolejje** — веб-приложение для отслеживания домашних заданий, оценок и базового анализа успеваемости.  
Создано, чтобы заменить бумажный дневник и упростить рутину ученика, учителя и родителя.

---

## 📋 Возможности

- **Регистрация и аутентификация** — личный кабинет для каждой роли (ученик, учитель, администратор)
- **Просмотр расписания и домашних заданий** — все задания с описанием и сроками сдачи
- **Выставление и просмотр оценок** — быстрый доступ к текущей успеваемости
- **Базовый анализ** — подсчёт среднего балла
- **Безопасный доступ** — JWT‑токены и хэширование паролей

---

## 🧱 Технологический стек

| Уровень                | Технологии                                      |
| ---------------------- | ----------------------------------------------- |
| **Frontend**           | HTML5, CSS3, JavaScript (Vanilla)               |
| **Backend**            | Python 3.10+, FastAPI, Pydantic                 |
| **База данных**        | PostgreSQL, asyncpg / SQLAlchemy (при наличии)  |
| **Аутентификация**     | JWT, bcrypt                                     |
| **Веб-сервер**         | uvicorn (dev), Nginx (production)               |
| **Статический сервер** | встроенный `http.server` (dev)                  |

---

## ⚙️ Требования

- Python 3.10 или выше
- PostgreSQL 14+ (локально или удалённо)
- `pip` для установки зависимостей
- Рекомендуемая ОС: **Linux** (Ubuntu 24.04+)

---

## 🚀 Быстрый старт (разработка)

### 1. Клонирование репозитория

```bash
git clone https://github.com/rival-afk/Nolejje
cd nolejje
```

### 2. Виртуальное окружение и зависимости

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

*Если файла `requirements.txt` нет, установите основные пакеты:*
```bash
pip install fastapi uvicorn python-dotenv asyncpg passlib[bcrypt] python-jose pydantic
```

### 3. Конфигурация окружения

Скопируйте файл примера и заполните **реальными** значениями:

```bash
mv .env.example .env
nano .env
```

**Содержимое `.env.example`** (замените на свои данные):

```
SECRET_KEY=12345
DATABASE_URL=postgresql://user:password@localhost/nolejje
```

### 4. База данных

Создайте пустую базу данных PostgreSQL:

```bash
sudo -u postgres psql -c "CREATE DATABASE nolejje;"
```

Примените схему и начальные данные из дампа (если он есть в проекте):

```bash
sudo -u postgres psql -d nolejje -f database.sql
```

### 5. Запуск

Проект состоит из двух процессов – **бэкенд (API)** и **фронтенд (статика)**.  
Запустите их в разных вкладках терминала.

**Запуск API (порт 5500):**

```bash
uvicorn main:app --host 0.0.0.0 --port 5500 --reload (--reload нужен для автоматической перезагрузки при изменении файлов апи)
```

**Запуск статики (порт 8000):**

```bash
python -m http.server 8000
```

Теперь приложение доступно по адресу:  
🌐 **http://localhost:8000** (веб-интерфейс)  
🔌 **http://localhost:5500** (API)  
📚 Документация API (автоматически): http://localhost:5500/docs

---

## 📁 Структура проекта

```
nolejje/
├── web/
│   ├── css/
│   │   ├── 404.css
│   │   ├── dashboard.css
│   │   ├── index.css
│   │   └── login.css
│   ├── img/
│   │   ├── icon.png
│   │   ├── nolejje_dark.png
│   │   └── nolejje_light.png
│   ├── js/
│   │   ├── auth.js
│   │   ├── dashboard.js
│   │   ├── index.js
│   │   └── login.js
│   ├── 404.html
│   ├── dashboard.html
│   ├── index.html
│   └── login.html
├── main.py
├── auth.py
├── db.py
├── jwt_handler.py
├── schemas.py
├── security.py
├── .env.example
├── .gitignore
├── version.json
├── README.md
├── url/
│   ├── redir.html
│   └── url.json
└── (другие служебные файлы)
```

Файлы и папки, не участвующие в передаче: `.git`, `.idea`, `.local-coder`, `.rtts`, `.venv`, `.vscode`, `__pycache__`, `docs`, `lib`.

---

## 🔐 Аутентификация

Используется механизм **JWT (JSON Web Tokens)**.
- При логине сервер выдаёт `access_token`.
- Токен сохраняется в `localStorage` браузера.
- При каждом защищённом запросе токен передаётся в заголовке `Authorization: Bearer <access_token>`.
- Пароли хранятся в базе в виде хэшей (bcrypt).
- Секретный ключ для подписи токенов задаётся в `.env` (`SECRET_KEY`).

---

## 🗄️ Работа с базой данных

### Восстановление базы из дампа

На целевой машине, после создания пустой БД `nolejje`, выполните:

```bash
sudo -u postgres psql -d nolejje -f database.sql
```

Убедитесь, что файл `database.sql` находится в корне проекта (или укажите полный путь).

---

## 🌐 Продакшен-размещение

Для реальной эксплуатации рекомендуется использовать связку **Nginx + Uvicorn**:

1. **Запуск FastAPI** (например, через systemd или supervisor)
   ```bash
   uvicorn main:app
   ```
2. **Настройка Nginx** как обратного прокси и раздатчика статики
```bash
sudo cat > /etc/nginx/sites-available/default << 'EOF'
server {
    listen 80;
    listen [::]:80;
    
    server_name localhost;
    
    root /home/itz-rival/projects/nolejje/web;
    index index.html;
    
    location / {
        try_files $uri $uri.html $uri/ =404;
    }
    
    error_page 404 /404.html;
    
    location = /404.html {
        internal;
    }
    
    location /api/ {
        proxy_pass http://127.0.0.1:8000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
```

Пример конфигурации Nginx можно найти в файле `nginx.example.conf` (если присутствует в проекте) либо создать по шаблону.

---

## 🔧 Версионирование

Текущая версия приложения хранится в файле `version.json`.  
Пример содержимого:
```json
{
  "version": "1.0.0",
  "date": "2026-05-01"
}
```

---

## ❓ Частые проблемы

**1. Peer authentication failed**  
При попытке дампа или подключения к PostgreSQL используйте `sudo -u postgres`.

**2. Порт занят**  
Проверьте список процессов (`sudo lsof -i :5500` или `sudo lsof -i :8000`) и освободите порт или измените порты в командах запуска.

**3. Не применяется схема БД**
Убедитесь, что база `nolejje` создана и команда `psql` выполняется от корректного пользователя с правами на базу.

**4. Статические файлы не загружаются**  
Убедитесь, что вы запустили `http.server` из корня проекта с флагом `--directory web`.  
В браузере откройте инструменты разработчика (F12 → Network / Сеть) – там видно, какие файлы не найдены.

---

## 📖 Лицензия

Проект создан в рамках учебной работы. Использование без согласования с автором не рекомендуется.

---

## 🤝 Контактная информация

Автор: **Иванович Кирилл Сергеевич**  
Руководитель: [ФИО руководителя]  
Учебное заведение: ГУО «Гимназия №11 города Минска имени И. Д. Черняховского»

---

> ✨ **Nolejje** – это шаг к современной школе, где технологии помогают учиться и учить.