# Nolejje — Электронный дневник

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8)](https://go.dev) [![Gin](https://img.shields.io/badge/Gin-1.12%2B-green)](https://gin-gonic.com) [![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14%2B-blue)](https://www.postgresql.org)  

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
| **Backend**            | Go 1.25+, Gin, pgx/pgxpool                      |
| **База данных**        | PostgreSQL 15+                                  |
| **Аутентификация**     | JWT (golang-jwt), bcrypt                        |
| **Веб-сервер**         | встроенный HTTP-сервер Go (dev), Nginx (prod)   |
| **Статический сервер** | встроенный `http.server` (dev)                  |

---

## ⚙️ Требования

- Go 1.25 или выше
- PostgreSQL 14+ (локально или удалённо)
- Рекомендуемая ОС: **Linux** (Ubuntu 24.04+)

---

## 🚀 Быстрый старт (разработка)

### 1. Клонирование репозитория

```bash
git clone https://github.com/rival-afk/Nolejje
cd nolejje
```

### 2. Сборка

```bash
go mod download
go build -o nolejje .
```

Зависимости подтянутся автоматически из `go.mod` (gin, pgxpool, golang-jwt, bcrypt, godotenv).

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

**Запуск API (порт 8000):**

```bash
go run .
```

**Запуск статики (порт 8000):**

```bash
python -m http.server 8000
```

Теперь приложение доступно по адресу:  
🌐 **http://localhost:8000** (веб-интерфейс)  
🔌 **http://localhost:8000** (API)

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
├── main.go
├── auth.go
├── db.go
├── jwt_handler.go
├── schemas.go
├── security.go
├── go.mod
├── go.sum
├── .env.example
├── .gitignore
├── version.json
├── README.md
├── Dockerfile
├── docker-compose.yaml
└── (другие служебные файлы)
```

Файлы и папки, не участвующие в передаче: `.git`, `.idea`, `.local-coder`, `.rtts`, `.venv`, `.vscode`, `__pycache__`, `docs`, `lib`.

---

## 🔐 Аутентификация

Используется механизм **JWT (JSON Web Tokens)**.
- При логине сервер выдаёт `access_token`.
- Токен выдаётся в ответе `access_token` и устанавливается сервером в защищённое хранилище браузера — HttpOnly-cookie (`Secure`, `SameSite=Strict`), она недоступна скриптам.
- Сервер принимает токен из заголовка `Authorization: Bearer <access_token>` или из cookie.
- При каждом защищённом запросе токен передаётся в заголовке `Authorization: Bearer <access_token>`.
- Пароли хранятся в базе в виде хэшей (bcrypt).
- Секретный ключ для подписи токенов задаётся в `.env` (`jwt_secret_key`).

---

## 🗄️ Работа с базой данных

### Восстановление базы из дампа

На целевой машине, после создания пустой БД `nolejje`, выполните:

```bash
sudo -u postgres psql -d nolejje -f database.sql
```

Убедитесь, что файл `database.sql` находится в корне проекта (или укажите полный путь).

---

## 🌐 Продакшен-развертывание

### Быстрый вариант (бинарник + systemd + Nginx)

```bash
# 1. Сборка (на любой машине; итог — статический бинарник для Linux x86_64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o nolejje .
# (или просто: make release)

# 2. Заливка на сервер
scp nolejje version.json web/ user@server:/opt/nolejje/

# 3. На сервере — окружение
cp .env.example /opt/nolejje/.env && nano /opt/nolejje/.env

# 4. Сборка базы
sudo -u postgres psql -d nolejje -f database.sql

# 5. systemd
sudo cp deploy/nolejje.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now nolejje
```

Шаблон Nginx — `deploy/nginx.example.conf` (статика из `web/`, `/api/` → 127.0.0.1:8000). За HttpOnly-cookie следит приложение: токен не хранится в localStorage, а cookie выставляет сервер.

### Docker-вариант

```bash
cp .env.example .env && nano .env
docker compose up -d --build
```

`docker-compose.yaml` поднимает приложение и PostgreSQL 15 (healthcheck, volume `postgres_data`).

### Проверка

- `GET /status` — сервер, бд, версия
- `GET /info` — версия и дата обновления
- `GET /docs` — авто-документация со всеми зарегистрированными маршрутами (для отправки запросов прямо из браузера)

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
Проверьте список процессов (`sudo lsof -i :8000` или `sudo lsof -i :8000`) и освободите порт или измените порты в командах запуска.

**3. Не применяется схема БД**
Убедитесь, что база `nolejje` создана и команда `psql` выполняется от корректного пользователя с правами на базу.

**4. Статические файлы не загружаются**  
В прод-варианте статику отдаёт Nginx из `web/` (см. `deploy/nginx.example.conf`); в дев-варианте запустите `python -m http.server --directory web` из корня проекта.  
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