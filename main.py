# FAST API
from fastapi import FastAPI, HTTPException, Depends
from fastapi.responses import JSONResponse
from fastapi.requests import Request
from fastapi.middleware.cors import CORSMiddleware
# внешние импорты
import time
import requests
import json
import base64
import asyncpg
from datetime import date
from typing import Optional

# из проекта
from db import create_pool, close_pool
import db
from schemas import Register, Login, GradePost, Assign, UserUpdate, AvatarUpdate, AdminCreate, ClassCreate, SubjectCreate
from security import hash_password, verify_password
from jwt_handler import create_access_token
from auth import get_user_func

app = FastAPI()
with open ("version.json") as file:
    VERSION_INFO = json.load(file)

# <! Middlewares !> #       (пока можно все. добавляю только ради обхода CORS)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

# <! Functions !> #         (для функций не связанных с эндпоинтами)

ALLOWED_AVATAR_MIME_TYPES = {
    "image/png",
    "image/jpeg",
    "image/jpg",
    "image/webp",
    "image/gif",
}

MAX_AVATAR_BYTES = 2 * 1024 * 1024

async def ensure_profile_columns():
    async with db.pool.acquire() as conn:
        await conn.execute(
            """
            ALTER TABLE users
            ADD COLUMN IF NOT EXISTS avatar_url TEXT
            """
        )

async def get_or_create_school_id(conn, school_name: str):
    normalized_name = school_name.strip()
    school = await conn.fetchrow(
        "SELECT id FROM schools WHERE LOWER(name) = LOWER($1)",
        normalized_name
    )

    if school:
        return school["id"]

    created_school = await conn.fetchrow(
        """
        INSERT INTO schools (name)
        VALUES ($1)
        RETURNING id
        """,
        normalized_name
    )
    return created_school["id"]

async def get_current_student_func(current_user=Depends(get_user_func)):
    async with db.pool.acquire() as conn:
        student = await conn.fetchrow("SELECT * FROM students WHERE user_id=$1", current_user["id"])
        
        if not student:
            raise HTTPException(status_code=404, detail="Мы перерыли всю бд, но так его и не нашли :(")
    return student

# <! Event handlers !> #

@app.on_event("startup")
async def startup():
    print("Запуск...")
    try:
        await create_pool()
        await ensure_profile_columns()
        print("Подключение к базе данных успешно")
    except Exception as e:
        print(f"Ошибка при запуске: {e}")
        raise

@app.on_event("shutdown")
async def shutdown():
    print("Завершение работы...")
    await close_pool()

# <! GET-запросы !> #       (да, я просто теряю время на красивом оформлении комментариев)
@app.get("/")
def root():
    raise HTTPException(
        status_code=204,
        detail="Nothing to do here"
    )

@app.get("/students/me/subjects")
async def get_subjects(current_user = Depends(get_user_func)):
    
    async with db.pool.acquire() as conn:
        subjects = await conn.fetch(
        """SELECT subjects.id, subjects.name FROM subjects
        JOIN students ON students.class_id = subjects.class_id
        WHERE students.user_id = $1""",
        current_user["id"]
    )
    
    return [dict(row) for row in subjects]

@app.get("/info")
def get_info():
    
    return {
        "name": "Nolejje",
        "version": VERSION_INFO["version"],
        "updated": VERSION_INFO["updated"]
    }

@app.get("/developer/age")
def get_developer_age():
    birth_date = date(2012, 3, 28)
    today = date.today()
    age = today.year - birth_date.year - ((today.month, today.day) < (birth_date.month, birth_date.day))
    return {
        "name": "itz_rival",
        "age": age,
        "stack": ["python", "js", "aiogram", "docker", "fastapi", "html", "css", "nginx"]
    }

@app.get("/students")
async def get_students(student_id: int):
    
    async with db.pool.acquire() as conn:
        students = await conn.fetch("""
            SELECT users.name FROM students
            JOIN users ON students.user_id = users.id
            WHERE students.id = $1""", student_id)
    
    if students == []:
        raise HTTPException(
            status_code=404,
            detail="No students in table :/"
        )
    return [dict(row) for row in students]

@app.get("/users/me")
async def get_current_user(current_user = Depends(get_user_func)):
    return current_user

@app.patch("/users/me")
async def update_current_user(payload: UserUpdate, current_user = Depends(get_user_func)):
    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Имя не может быть пустым")

    async with db.pool.acquire() as conn:
        user = await conn.fetchrow(
            """
            UPDATE users
            SET name = $1
            WHERE id = $2
            RETURNING *
            """,
            name,
            current_user["id"]
        )

    return user

@app.post("/users/me/avatar")
async def update_avatar(payload: AvatarUpdate, current_user = Depends(get_user_func)):
    avatar_url = payload.avatar_url.strip()

    if not avatar_url.startswith("data:") or ";base64," not in avatar_url:
        raise HTTPException(status_code=400, detail="Некорректный формат аватара")

    header, encoded = avatar_url.split(",", 1)
    mime_type = header[5:].split(";")[0].lower()

    if mime_type not in ALLOWED_AVATAR_MIME_TYPES:
        raise HTTPException(status_code=400, detail="Поддерживаются PNG, JPG, WEBP и GIF")

    try:
        file_bytes = base64.b64decode(encoded, validate=True)
    except Exception:
        raise HTTPException(status_code=400, detail="Не удалось декодировать изображение")

    if len(file_bytes) > MAX_AVATAR_BYTES:
        raise HTTPException(status_code=400, detail="Максимальный размер аватара — 2 МБ")

    async with db.pool.acquire() as conn:
        user = await conn.fetchrow(
            """
            UPDATE users
            SET avatar_url = $1
            WHERE id = $2
            RETURNING avatar_url
            """,
            avatar_url,
            current_user["id"]
        )

    return {"avatar_url": user["avatar_url"]}

@app.delete("/users/me/avatar")
async def delete_avatar(current_user = Depends(get_user_func)):
    async with db.pool.acquire() as conn:
        await conn.execute(
            """
            UPDATE users
            SET avatar_url = NULL
            WHERE id = $1
            """,
            current_user["id"]
        )

    return {"status": "successful"}

@app.get("/students/me")
async def get_current_student(student = Depends(get_current_student_func)):
    student_id = student["id"]
    class_id = student["class_id"]
    async with db.pool.acquire() as conn:
        student_data = await conn.fetchrow("""
        SELECT classes.number || ' «' || classes.letter || '»' AS class, schools.name AS school_name, COALESCE(AVG(g.grade), 0) AS avg_grade FROM students
        JOIN classes ON students.class_id = classes.id
        JOIN schools ON classes.school_id = schools.id
        LEFT JOIN grades g ON students.id = g.student_id
        WHERE students.id = $1
        GROUP BY classes.number, classes.letter, schools.name
        """,
        student["id"])
    return {"student_id": student_id, "class_id": class_id, "class": student_data["class"], "school_name": student_data["school_name"], "avg_grade": student_data["avg_grade"]}

@app.get("/students/me/homeworks")
async def get_homeworks (limit: Optional[int] = None, offset: Optional[int] = None, only_active: Optional[bool] = None, student = Depends(get_current_student_func)):
    
    async with db.pool.acquire() as conn:
        
        if not student:
            raise HTTPException(
                status_code=404,
                detail="Student not found :\\"
            )
        
        params = [student["class_id"]]
        
        order = " ORDER BY h.due_date ASC"
        only_active_query = " AND h.due_date >= CURRENT_DATE"
        
        request = """
            SELECT h.id, h.description, h.title, h.due_date, su.name as subject_name FROM students st
            JOIN classes c ON st.class_id = c.id
            JOIN subjects su ON c.id = su.class_id
            JOIN homeworks h ON h.subject_id = su.id
            WHERE st.class_id = $1
            """
        
        if only_active:
            request += only_active_query
        
        request += order
        
        if limit != None:
            limit_query = f" LIMIT ${len(params) + 1}"
            request += limit_query
            params.append(limit)
        
        if offset != None:
            offset_query = f" OFFSET ${len(params) + 1}"
            request += offset_query
            params.append(offset)
        
        homeworks = await conn.fetch(
            request,
            *params
            )
    
    return [dict(row) for row in homeworks]

@app.get("/homework")
async def get_homework (subject_id: int, current_user = Depends(get_user_func)):
    
    async with db.pool.acquire() as conn:
        
        class_id = await conn.fetchrow("""
            SELECT class_id FROM students WHERE user_id = $1
            """,
            current_user["id"]
            )
        
        if class_id == None:
            raise HTTPException(
                status_code=404,
                detail="Not Found"
            )
        
        subject_class_id = await conn.fetchrow("""
            SELECT class_id FROM subjects WHERE id = $1 
            """,
            subject_id
            )
        
        if subject_class_id == None:
            raise HTTPException(
                status_code=404,
                detail="Not Found"
            )
        
        if class_id["class_id"] == subject_class_id["class_id"]:
            homework = await conn.fetchrow("""
                SELECT id, title, description, due_date FROM homeworks
                WHERE subject_id = $1
                ORDER BY due_date ASC
            """, subject_id
            )
        else:
            raise HTTPException(
                status_code=403,
                detail="Forbidden \\:-|"
            )
    
    return homework

@app.get("/ping")
async def get_ping():
    
    before = time.time_ns()
    
    try:
        url = "https://google.com"
        
        response = requests.get(url)
        
        if response.status_code == 200:
            pass
        else:
            print(f"Произошла ошибка: {response.status_code}")
    except requests.exceptions.RequestException as e:
        print("An error occured while requesting ", url)
    
    after = time.time_ns()
    
    difference = (after - before) / 1000000
    
    rounded_ds = round(difference)
    return {
        "message": "Pong!",
        "ping": f"{rounded_ds} ms"
    }

@app.get("/status")
async def get_status():
    
    server = "online"
    
    db_status = "down"
    
    try:
        async with db.pool.acquire() as conn:
            result = await conn.fetchrow("""
                SELECT 1
            """)
        
        db_status = "available"
        
    except:
        db_status = "down"
    
    if db_status == "available":
        status = "ok"
    else:
        status = "error"
    
    return {"status": status, "server": server, "db": db_status, "version": VERSION_INFO["version"]}

@app.get("/classes")
async def get_classes():
    async with db.pool.acquire() as conn:
        
        classes_list = await conn.fetch("""
            SELECT classes.id, schools.name, classes.number || classes.letter AS class FROM classes
            JOIN schools ON schools.id = classes.school_id
            """)
    
    return [dict(row) for row in classes_list]

@app.get("/auth/refresh")
async def refresh_token(current_user = Depends(get_user_func)):
    new_token = create_access_token(
        {"user_id": current_user["id"]}
    )
    return {"new_token": new_token}

@app.get("/teacher/classes")
async def get_teacher_classes(current_user = Depends(get_user_func)):
    
    if current_user["role"] != "teacher":
        raise HTTPException(
            status_code=403,
            detail="Forbidden. You aren\'t a teacher"
        )
    
    async with db.pool.acquire() as conn:
        teacher_id = await conn.fetchrow("""
            SELECT id FROM teachers WHERE user_id =$1
            """, current_user["id"])
        
        if not teacher_id:
            raise HTTPException(status_code=404, detail="Учитель не найден в базе")
        
        classes = await conn.fetch("""
            SELECT classes.id, classes.number || classes.letter AS name, schools.name AS school_name, COUNT(students.id) AS students_count FROM classes
            JOIN schools ON classes.school_id = schools.id
            JOIN subjects ON subjects.class_id = classes.id
            LEFT JOIN students ON students.class_id = classes.id
            WHERE subjects.teacher_id = $1
            GROUP BY classes.id, classes.number, classes.letter, schools.name
            """, teacher_id["id"])
        
    return [dict(row) for row in classes]

@app.get("/admin/assignments")
async def get_assignments(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")
    
    async with db.pool.acquire() as conn:
        list = await conn.fetch("""
            SELECT 
                tcs.id,
                users.name as teacher,
                classes.number || classes.letter as class,
                subjects.name as subject
            FROM teacher_class_subjects tcs
            JOIN teachers ON tcs.teacher_id = teachers.id
            JOIN users ON teachers.user_id = users.id
            JOIN classes ON tcs.class_id = classes.id
            JOIN subjects ON tcs.subject_id = subjects.id
            """)
    
    return [dict(row) for row in list]

@app.get("/admin/classes")
async def get_admin_classes(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")
    
    async with db.pool.acquire() as conn:
        classes = await conn.fetch("""
            SELECT classes.id, classes.number || classes.letter AS name, COUNT(students.id) AS students_count FROM classes
            JOIN schools ON schools.id = classes.school_id
            LEFT JOIN students ON students.class_id = classes.id
            GROUP BY classes.id, classes.number, classes.letter, schools.name
            """)
    
    return [dict(row) for row in classes]

@app.get("/admin/schools")
async def get_admin_schools(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    async with db.pool.acquire() as conn:
        schools = await conn.fetch("""
            SELECT id, name
            FROM schools
            ORDER BY name
            """)

    return [dict(row) for row in schools]

@app.get("/admin/analytics")
async def get_admin_analytics(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    async with db.pool.acquire() as conn:
        analytics = await conn.fetch("""
            SELECT
                (SELECT COUNT(*) FROM users) AS total_users,
                (SELECT COUNT(*) FROM users WHERE role = 'student') AS total_students,
                (SELECT COUNT(*) FROM users WHERE role = 'teacher') AS total_teachers,
                (SELECT COUNT(*) FROM users WHERE role = 'admin') AS total_admins,
                (SELECT COUNT(*) FROM classes) AS total_classes,
                (SELECT COALESCE(AVG(grades.grade), 0) FROM grades) AS avg_grade
            """)
    
    return [dict(row) for row in analytics]

@app.get("/admin/subjects")
async def get_admin_subjects(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    async with db.pool.acquire() as conn:
        subjects = await conn.fetch("""
            SELECT subjects.id, subjects.name, classes.number || classes.letter AS class FROM subjects
            JOIN classes ON subjects.class_id = classes.id
            """)
    
    return [dict(row) for row in subjects]

@app.get("/admin/teachers")
async def get_admin_teachers(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    async with db.pool.acquire() as conn:
        teachers = await conn.fetch("""
            SELECT teachers.id, users.name FROM teachers
            JOIN users ON teachers.user_id = users.id
            """)

    return [dict(row) for row in teachers]

@app.get("/admin/admins")
async def get_admin_admins(current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    async with db.pool.acquire() as conn:
        admins = await conn.fetch("""
            SELECT id, name, email
            FROM users
            WHERE role = 'admin'
            ORDER BY name
            """)

    return [dict(row) for row in admins]

@app.get("/students/me/grades")
async def get_grades(current_user = Depends(get_user_func)):
    if current_user["role"] != "student":
        raise HTTPException(status_code=403, detail="Forbidden")

    async with db.pool.acquire() as conn:
        grades = await conn.fetch("""
            SELECT grades.id, subjects.name AS subject_name, grades.grade AS value, grades.date, grades.lesson_id
            FROM grades
            JOIN subjects ON grades.subject_id = subjects.id
            JOIN students ON grades.student_id = students.id
            WHERE students.user_id = $1
            ORDER BY grades.date DESC
            """, current_user["id"])

    return [dict(row) for row in grades]

@app.get("/students/me/schedule")
async def get_schedule(current_user = Depends(get_user_func)):
    if current_user["role"] != "student":
        raise HTTPException(status_code=403, detail="Forbidden")

    try:
        async with db.pool.acquire() as conn:
            schedule = await conn.fetch("""
                SELECT 
                    lessons.weekday,
                    subjects.name AS subject_name,
                    users.name AS teacher_name,
                    lessons.room
                FROM lessons
                JOIN subjects ON lessons.subject_id = subjects.id
                JOIN teachers ON lessons.teacher_id = teachers.id
                JOIN users ON teachers.user_id = users.id
                JOIN students ON lessons.class_id = students.class_id
                WHERE students.user_id = $1
                ORDER BY lessons.weekday, lessons.time
            """, current_user["id"])
    except asyncpg.UndefinedTableError:
        return []

    return [dict(row) for row in schedule]

@app.get("/teacher/classes/{class_id}/grades")
async def get_class_grades(class_id: int, current_user = Depends(get_user_func)):
    if current_user["role"] != "teacher":
        raise HTTPException(status_code=403, detail="Forbidden")

    async with db.pool.acquire() as conn:
        grades = await conn.fetch("""
            SELECT 
                students.id AS student_id,
                users.name AS student_name,
                subjects.name AS subject_name,
                grades.grade AS value,
                grades.date,
                grades.id AS grade_id
            FROM grades
            JOIN students ON grades.student_id = students.id
            JOIN users ON students.user_id = users.id
            JOIN subjects ON grades.subject_id = subjects.id
            WHERE students.class_id = $1
            ORDER BY grades.date DESC
        """, class_id)

    return [dict(row) for row in grades]

@app.get("/api/teacher/schedule")
async def get_teacher_schedule(current_user = Depends(get_user_func)):
    if current_user["role"] != "teacher":
        raise HTTPException(status_code=403, detail="Forbidden")

    try:
        async with db.pool.acquire() as conn:
            schedule = await conn.fetch(
                """
                SELECT
                    lessons.weekday,
                    classes.number || classes.letter AS class_name,
                    subjects.name AS subject_name,
                    lessons.room
                FROM lessons
                JOIN subjects ON lessons.subject_id = subjects.id
                JOIN classes ON lessons.class_id = classes.id
                JOIN teachers ON lessons.teacher_id = teachers.id
                WHERE teachers.user_id = $1
                ORDER BY lessons.weekday, lessons.time
                """,
                current_user["id"]
            )
    except asyncpg.UndefinedTableError:
        return []

    return [dict(row) for row in schedule]

# <! DELETE-запросы!> #

@app.delete("/users/me")
async def delete_current_user(current_user = Depends(get_user_func)):
    async with db.pool.acquire() as conn:
        async with conn.transaction():
            if current_user["role"] == "student":
                await conn.execute(
                    "DELETE FROM students WHERE user_id = $1",
                    current_user["id"]
                )
            elif current_user["role"] == "teacher":
                teacher = await conn.fetchrow(
                    "SELECT id FROM teachers WHERE user_id = $1",
                    current_user["id"]
                )
                if teacher:
                    await conn.execute(
                        "DELETE FROM teacher_class_subjects WHERE teacher_id = $1",
                        teacher["id"]
                    )
                    await conn.execute(
                        "UPDATE subjects SET teacher_id = NULL WHERE teacher_id = $1",
                        teacher["id"]
                    )
                    await conn.execute(
                        "DELETE FROM teachers WHERE user_id = $1",
                        current_user["id"]
                    )

            deleted = await conn.execute(
                "DELETE FROM users WHERE id = $1",
                current_user["id"]
            )

    if deleted.endswith("0"):
        raise HTTPException(status_code=404, detail="Пользователь не найден")

    return {"status": "successful"}

@app.delete("/admin/assign/{id}")
async def delete_assignment(id: int, current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")
    
    async with db.pool.acquire() as conn:
        existing = await conn.fetchrow("""SELECT id FROM teacher_class_subjects WHERE id = $1""", id)
        
        if not existing:
            raise HTTPException(status_code=404, detail=f"Assignment {id} Not Found")
        
        await conn.execute("""
            DELETE FROM teacher_class_subjects WHERE id = $1""", id)
    return {"status": "successful"}

@app.delete("/api/teacher/grades/{grade_id}")
async def delete_grade(grade_id: int, current_user = Depends(get_user_func)):
    if current_user["role"] != "teacher":
        raise HTTPException(status_code=403, detail="Forbidden")

    async with db.pool.acquire() as conn:
        teacher = await conn.fetchrow(
            "SELECT id FROM teachers WHERE user_id = $1",
            current_user["id"]
        )

        if not teacher:
            raise HTTPException(status_code=403, detail="Forbidden")

        grade = await conn.fetchrow(
            """
            SELECT g.id FROM grades g
            JOIN subjects s ON g.subject_id = s.id
            WHERE g.id = $1 AND s.teacher_id = $2
            """,
            grade_id,
            teacher["id"]
        )

        if not grade:
            existing = await conn.fetchrow(
                "SELECT id FROM grades WHERE id = $1",
                grade_id
            )
            if not existing:
                raise HTTPException(status_code=404, detail=f"Grade {grade_id} Not Found")
            raise HTTPException(status_code=403, detail="Forbidden")

        await conn.execute(
            "DELETE FROM grades WHERE id = $1",
            grade_id
        )

    return {"status": "successful"}

# <! POST-запросы!> #

@app.post("/admin/users")
async def create_admin_user(payload: AdminCreate, current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    name = payload.name.strip()
    email = payload.email.strip().lower()
    password = payload.password

    if not name or not email or not password:
        raise HTTPException(status_code=400, detail="Нужно заполнить имя, почту и пароль")

    async with db.pool.acquire() as conn:
        existing_user = await conn.fetchrow(
            "SELECT id FROM users WHERE email = $1",
            email
        )
        if existing_user:
            raise HTTPException(status_code=400, detail="Email already registered")

        password_hash = hash_password(password)
        created_user = await conn.fetchrow(
            """
            INSERT INTO users (name, email, passwd_hash, role)
            VALUES ($1, $2, $3, 'admin')
            RETURNING id, name, email, role
            """,
            name,
            email,
            password_hash
        )

    return dict(created_user)

@app.post("/admin/classes")
async def create_admin_class(payload: ClassCreate, current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    letter = payload.letter.strip().upper()
    school_name = payload.school_name.strip()

    if payload.number <= 0:
        raise HTTPException(status_code=400, detail="Номер класса должен быть больше нуля")
    if not letter:
        raise HTTPException(status_code=400, detail="Буква класса обязательна")
    if not school_name:
        raise HTTPException(status_code=400, detail="Укажите школу")

    async with db.pool.acquire() as conn:
        school_id = await get_or_create_school_id(conn, school_name)

        existing_class = await conn.fetchrow(
            """
            SELECT id FROM classes
            WHERE number = $1 AND UPPER(letter) = $2 AND school_id = $3
            """,
            payload.number,
            letter,
            school_id
        )
        if existing_class:
            raise HTTPException(status_code=400, detail="Такой класс уже существует")

        created_class = await conn.fetchrow(
            """
            INSERT INTO classes (number, letter, school_id)
            VALUES ($1, $2, $3)
            RETURNING id, number, letter, school_id
            """,
            payload.number,
            letter,
            school_id
        )

    return dict(created_class)

@app.post("/admin/subjects")
async def create_admin_subject(payload: SubjectCreate, current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")

    name = payload.name.strip()
    if not name:
        raise HTTPException(status_code=400, detail="Название предмета обязательно")

    async with db.pool.acquire() as conn:
        class_exists = await conn.fetchrow(
            "SELECT id FROM classes WHERE id = $1",
            payload.class_id
        )
        if not class_exists:
            raise HTTPException(status_code=400, detail="Класс не найден")

        existing_subject = await conn.fetchrow(
            """
            SELECT id FROM subjects
            WHERE LOWER(name) = LOWER($1) AND class_id = $2
            """,
            name,
            payload.class_id
        )
        if existing_subject:
            raise HTTPException(status_code=400, detail="Такой предмет уже есть у этого класса")

        created_subject = await conn.fetchrow(
            """
            INSERT INTO subjects (name, class_id)
            VALUES ($1, $2)
            RETURNING id, name, class_id
            """,
            name,
            payload.class_id
        )

    return dict(created_subject)

@app.post("/auth/register")
async def register(user: Register):
    async with db.pool.acquire() as conn:
        existing_user = await conn.fetchrow(
            "SELECT id FROM users WHERE email = $1", user.email
        )
        if existing_user:
            raise HTTPException(status_code=400, detail="Email already registered")

        password_hash = hash_password(user.password)

        if user.role not in ('student', 'teacher'):
            raise HTTPException(status_code=400, detail="Incorrect role")

        if user.role == 'student':
            if user.class_id is None:
                raise HTTPException(status_code=400, detail="Class ID is required for this role")
            class_exists = await conn.fetchval(
                "SELECT 1 FROM classes WHERE id = $1", user.class_id
            )
            if not class_exists:
                raise HTTPException(status_code=400, detail="Class doesn't exist")

        id_record = await conn.fetchrow(
            """
            INSERT INTO users (name, email, passwd_hash, role)
            VALUES ($1, $2, $3, $4)
            RETURNING id
            """,
            user.name, user.email, password_hash, user.role
        )
        user_id = id_record["id"]

        if user.role == 'student':
            await conn.execute(
                "INSERT INTO students (user_id, class_id) VALUES ($1, $2)",
                user_id, user.class_id
            )
        elif user.role == 'teacher':
            await conn.execute(
                "INSERT INTO teachers (user_id) VALUES ($1)", user_id
            )

    return {"status": "successful"}

@app.post("/auth/login")
async def login(user: Login):
    
    async with db.pool.acquire() as conn:
        db_user = await conn.fetchrow(
        "SELECT * FROM users WHERE email = $1", user.email
    )
    
    if not db_user:
        raise HTTPException(
            status_code=401,
            detail="Invalid email or password"
        )
    
    if not verify_password(user.password, db_user["passwd_hash"]):
        raise HTTPException(
            status_code=401,
            detail="Invalid email or password"
        )
    
    token = create_access_token(
        {"user_id": db_user["id"]}
    )
    
    return {"access_token": token}

@app.post("/api/teacher/grades")
async def grades (grade_data: dict, current_user = Depends(get_user_func)):
    if current_user["role"] != "teacher":
        raise HTTPException(
            status_code=403,
            detail="Forbidden :|"
        )

    student_id = grade_data.get("student_id")
    subject_id = grade_data.get("subject_id")
    value = grade_data.get("value")
    date = grade_data.get("date")

    if student_id is None or value is None or date is None:
        raise HTTPException(
            status_code=400,
            detail="Missing required grade fields"
        )

    async with db.pool.acquire() as conn:
        teacher_id = await conn.fetchrow("""
            SELECT id FROM teachers
            WHERE user_id = $1
            """,
            current_user["id"]
            )
        
        if teacher_id == None:
            raise HTTPException(
                status_code=403,
                detail="Forbidden. You aren't a teacher"
            )
        
        if subject_id is None:
            student_class_id = await conn.fetchrow("""
                SELECT class_id FROM students WHERE id = $1
                """,
                student_id
                )

            if student_class_id == None:
                raise HTTPException(
                    status_code=404,
                    detail="Not Found"
                )

            subject_teacher_class_id = await conn.fetchrow("""
                SELECT id, class_id, teacher_id FROM subjects
                WHERE class_id = $1 AND teacher_id = $2
                ORDER BY id
                LIMIT 1
                """,
                student_class_id["class_id"],
                teacher_id["id"]
                )
            if subject_teacher_class_id:
                subject_id = subject_teacher_class_id["id"]
        else:
            subject_teacher_class_id = await conn.fetchrow("""
                SELECT id, class_id, teacher_id FROM subjects WHERE id = $1
                """,
                subject_id
                )
        
        if subject_teacher_class_id == None:
            raise HTTPException(
                status_code=404,
                detail="Subject not found for this teacher and class"
            )
        
        if teacher_id["id"] != subject_teacher_class_id["teacher_id"]:
            raise HTTPException(
                status_code=403,
                detail="Forbidden :|"
            )
        
        student_class_id = await conn.fetchrow("""
            SELECT class_id FROM students WHERE id = $1
            """,
            student_id
            )
        
        if student_class_id == None:
            raise HTTPException(
                status_code=404,
                detail="Not Found"
            )
        
        if subject_teacher_class_id["class_id"] != student_class_id["class_id"]:
            raise HTTPException(
                status_code=403,
                detail="Forbidden :|"
            )
        
        await conn.execute(
        """
        INSERT INTO grades (student_id, subject_id, grade, date)
        VALUES ($1, $2, $3, $4)
        """,
        student_id,
        subject_id,
        value,
        date
    )
    
    return {"status": "successful"}

@app.post("/select_class")
async def post_get_class(class_id: int, current_user = Depends(get_user_func)):
    
    if current_user["role"] != 'teacher':
        raise HTTPException(
            status_code=403,
            detail="Forbidden"
        )
    
    async with db.pool.acquire() as conn:
        teacher_class_id = await conn.fetchrow("""
        SELECT class_id FROM teachers
        WHERE user_id = $1
        """,
        current_user["id"])
        
        if teacher_class_id["class_id"] != None:
            raise HTTPException(
                status_code=400,
                detail="У вас уже есть класс"
            )
        
        class_exist = await conn.fetchrow("""
        SELECT 1 FROM classes WHERE class_id = $1
        """,
        class_id)
        
        if class_exist != 1:
            raise HTTPException(
                status_code=400,
                detail="Class doesn't exist"
            )
        
        await conn.execute("""
        UPDATE teachers SET class_id = $1 WHERE user_id = $2;
        """,
        class_id,
        current_user["id"])
    
    return {"status": "successful"}

@app.post("/admin/assign")
async def post_assign(assign: Assign, current_user = Depends(get_user_func)):
    if current_user["role"] != "admin":
        raise HTTPException(status_code=403, detail="Admin Only Endpoint")
    
    async with db.pool.acquire() as conn:
        assignment = await conn.fetchrow("""
            INSERT INTO teacher_class_subjects (teacher_id, class_id, subject_id)
            VALUES ($1, $2, $3)
            RETURNING id
            """,
            assign.teacher_id, assign.class_id, assign.subject_id)
    
    return assignment["id"]

# <! Error Handlers !> #        my English is very well)

@app.exception_handler(404)
async def not_found_handler(request: Request, exc):
    if hasattr(exc, 'detail') and exc.detail:
        return JSONResponse(
            status_code=404,
            content={
            "error": "Route Not Found :(",
            "detail": exc.detail,
            "path": request.url.path,
            }
        )
    
    return JSONResponse(
        status_code=404,
        content={
            "error": "Not Found :(",
            "path": request.url.path,
            "message": f"Route '{request.url.path}' not found"
        }
    )
