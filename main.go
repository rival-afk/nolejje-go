package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var VERSION_INFO map[string]any

var ALLOWED_AVATAR_MIME_TYPES = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/jpg":  true,
	"image/webp": true,
	"image/gif":  true,
}

const MAX_AVATAR_BYTES = 2 * 1024 * 1024

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "*")
		c.Header("Access-Control-Allow-Headers", "*")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}

func handleErr(c *gin.Context, err error) {
	apiErr, ok := err.(*ApiError)
	if !ok {
		log.Printf("Внутренняя ошибка на %s: %v", c.Request.URL.Path, err)
		apiErr = HttpError(http.StatusInternalServerError, "Internal Server Error")
	}
	if apiErr.Code == http.StatusNotFound && apiErr.Detail != "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route Not Found :(", "detail": apiErr.Detail, "path": c.Request.URL.Path})
		return
	}
	c.JSON(apiErr.Code, gin.H{"detail": apiErr.Detail})
}

func ManyRows(sql string, args ...any) ([]map[string]any, error) {
	rows, err := Pool.Query(Ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	many, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, err
	}
	if many == nil {
		many = []map[string]any{}
	}
	return many, nil
}

type Querier interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
}

func OneRowOn(q Querier, sql string, args ...any) (map[string]any, error) {
	rows, err := q.Query(Ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := pgx.CollectOneRow(rows, pgx.RowToMap)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func OneRow(sql string, args ...any) (map[string]any, error) {
	return OneRowOn(Pool, sql, args...)
}

func ensureProfileColumns() error {
	_, err := Pool.Exec(Ctx, "ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT")
	return err
}

func getOrCreateSchoolID(pgxTx pgx.Tx, schoolName string) (int64, error) {
	normalized_name := strings.TrimSpace(schoolName)
	school, err := OneRowOn(pgxTx, "SELECT id FROM schools WHERE LOWER(name) = LOWER($1)", normalized_name)
	if err != nil {
		return 0, err
	}
	if school != nil {
		return ToInt64(school["id"]), nil
	}
	created, err := OneRowOn(pgxTx, "INSERT INTO schools (name) VALUES ($1) RETURNING id", normalized_name)
	if err != nil {
		return 0, err
	}
	return ToInt64(created["id"]), nil
}

func getCurrentStudentFunc(c *gin.Context) (map[string]any, error) {
	current_user := CurrentUser(c)
	student, err := OneRow("SELECT * FROM students WHERE user_id=$1", ToInt64(current_user["id"]))
	if err != nil {
		return nil, err
	}
	if student == nil {
		return nil, HttpError(http.StatusNotFound, "Мы перерыли всю бд, но так его и не нашли :(")
	}
	return student, nil
}

func main() {
	versionData, err := os.ReadFile("version.json")
	if err == nil {
		if err = json.Unmarshal(versionData, &VERSION_INFO); err != nil {
			log.Println("Ошибка разбора version.json:", err)
		}
	} else {
		log.Println("Ошибка чтения version.json:", err)
	}

	log.Println("Запуск...")
	if err := CreatePool(); err != nil {
		log.Printf("Ошибка при запуске: %v", err)
		panic(err)
	}
	if err := ensureProfileColumns(); err != nil {
		log.Printf("Ошибка при запуске: %v", err)
		panic(err)
	}
	log.Println("Подключение к базе данных успешно")

	r := gin.New()
	Router = r
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	public := r
	protected := r.Group("/")
	protected.Use(Auth())

	registerGET(protected, public)
	registerOther(protected)
	registerDocs(public, r)

	r.NoRoute(notFoundHandler)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{Addr: "0.0.0.0:8000", Handler: r, BaseContext: func(net.Listener) context.Context { return Ctx }}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("Ошибка сервера:", err)
		}
	}()

	<-done
	log.Println("Завершение работы...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Ошибка завершения: %v", err)
	}
	ClosePool()
}

func registerGET(p gin.IRouter, public gin.IRouter) {
	r := p
	public.GET("/", func(c *gin.Context) {
		if webIndexHTTP(c.Writer, c.Request) { return }
		c.JSON(http.StatusNoContent, gin.H{"detail": "Nothing to do here"})
	})

	r.GET("/students/me/subjects", func(c *gin.Context) {
		current_user := CurrentUser(c)
		rows, err := ManyRows(`SELECT subjects.id, subjects.name FROM subjects
		JOIN students ON students.class_id = subjects.class_id
		WHERE students.user_id = $1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	public.GET("/info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"name": "Nolejje", "version": VERSION_INFO["version"], "updated": VERSION_INFO["updated"]})
	})

	public.GET("/developer/age", func(c *gin.Context) {
		now := time.Now()
		age := now.Year() - 2012
		if now.Month() < time.March || (now.Month() == time.March && now.Day() < 28) {
			age--
		}
		c.JSON(http.StatusOK, gin.H{"name": "itz_rival", "age": age, "stack": []string{"python", "js", "aiogram", "docker", "fastapi", "html", "css", "nginx"}})
	})

	public.GET("/students", func(c *gin.Context) {
		studentID, ok := QueryInt(c, "student_id")
		if !ok {
			return
		}
		students, err := ManyRows(`SELECT users.name FROM students
			JOIN users ON students.user_id = users.id
			WHERE students.id = $1`, studentID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if len(students) == 0 {
			handleErr(c, HttpError(http.StatusNotFound, "No students in table :/"))
			return
		}
		c.JSON(http.StatusOK, students)
	})

	r.GET("/users/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, CurrentUser(c))
	})

	r.GET("/students/me", func(c *gin.Context) {
		student, err := getCurrentStudentFunc(c)
		if err != nil {
			handleErr(c, err)
			return
		}
		studentID := ToInt64(student["id"])
		classID := ToInt64(student["class_id"])
		studentData, err := OneRow(`SELECT classes.number || ' «' || classes.letter || '»' AS class, schools.name AS school_name, COALESCE(AVG(g.grade), 0) AS avg_grade FROM students
		JOIN classes ON students.class_id = classes.id
		JOIN schools ON classes.school_id = schools.id
		LEFT JOIN grades g ON students.id = g.student_id
		WHERE students.id = $1
		GROUP BY classes.number, classes.letter, schools.name`, studentID)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"student_id": studentID, "class_id": classID, "class": studentData["class"], "school_name": studentData["school_name"], "avg_grade": studentData["avg_grade"]})
	})

	r.GET("/students/me/homeworks", func(c *gin.Context) {
		student, err := getCurrentStudentFunc(c)
		if err != nil {
			handleErr(c, err)
			return
		}
		limit := QueryOptionalInt64(c, "limit")
		offset := QueryOptionalInt64(c, "offset")
		onlyActive := QueryOptionalBool(c, "only_active")

		var request strings.Builder
		request.WriteString(`SELECT h.id, h.description, h.title, h.due_date, su.name as subject_name FROM students st
		JOIN classes c ON st.class_id = c.id
		JOIN subjects su ON c.id = su.class_id
		JOIN homeworks h ON h.subject_id = su.id
		WHERE st.class_id = $1`)
		if onlyActive {
			request.WriteString(" AND h.due_date >= CURRENT_DATE")
		}
		request.WriteString(" ORDER BY h.due_date ASC")

		args := []any{ToInt64(student["class_id"])}
		if limit != nil {
			request.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)+1))
			args = append(args, *limit)
		}
		if offset != nil {
			request.WriteString(fmt.Sprintf(" OFFSET $%d", len(args)+1))
			args = append(args, *offset)
		}
		rows, err := ManyRows(request.String(), args...)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/homework", func(c *gin.Context) {
		current_user := CurrentUser(c)
		subjectID, ok := QueryInt(c, "subject_id")
		if !ok {
			return
		}
		classID, err := OneRow(`SELECT class_id FROM students WHERE user_id = $1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if classID == nil {
			handleErr(c, HttpError(http.StatusNotFound, "Not Found"))
			return
		}
		subjectClassID, err := OneRow(`SELECT class_id FROM subjects WHERE id = $1`, subjectID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if subjectClassID == nil {
			handleErr(c, HttpError(http.StatusNotFound, "Not Found"))
			return
		}
		if ToInt64(classID["class_id"]) != ToInt64(subjectClassID["class_id"]) {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden \\:-|"))
			return
		}
		homework, err := OneRow(`SELECT id, title, description, due_date FROM homeworks
			WHERE subject_id = $1
			ORDER BY due_date ASC`, subjectID)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, homework)
	})

	public.GET("/ping", func(c *gin.Context) {
		before := time.Now().UnixNano()
		client := http.Client{Timeout: time.Minute}
		resp, err := client.Get("https://google.com")
		if err != nil {
			log.Println("An error occured while requesting", "https://google.com")
		} else if resp.StatusCode != 200 {
			log.Printf("Произошла ошибка: %d", resp.StatusCode)
		} else {
			resp.Body.Close()
		}
		after := time.Now().UnixNano()
		difference := float64(after-before) / 1000000.0
		rounded := int64(difference + 0.5)
		c.JSON(http.StatusOK, gin.H{"message": "Pong!", "ping": fmt.Sprintf("%d ms", rounded)})
	})

	public.GET("/status", func(c *gin.Context) {
		dbStatus := "down"
		_, err := OneRow("SELECT 1")
		if err == nil {
			dbStatus = "available"
		}
		status := "error"
		if dbStatus == "available" {
			status = "ok"
		}
		c.JSON(http.StatusOK, gin.H{"status": status, "server": "online", "db": dbStatus, "version": VERSION_INFO["version"]})
	})

	public.GET("/classes", func(c *gin.Context) {
		rows, err := ManyRows(`SELECT classes.id, schools.name, classes.number || classes.letter AS class FROM classes
			JOIN schools ON schools.id = classes.school_id`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/auth/refresh", func(c *gin.Context) {
		current_user := CurrentUser(c)
		newToken, err := CreateAccessToken(map[string]any{"user_id": ToInt64(current_user["id"])})
		if err != nil {
			handleErr(c, err)
			return
		}
		setAuthCookie(c, newToken)
		c.JSON(http.StatusOK, gin.H{"new_token": newToken})
	})

	r.GET("/teacher/classes", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden. You aren't a teacher"))
			return
		}
		teacherID, err := OneRow(`SELECT id FROM teachers WHERE user_id =$1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if teacherID == nil {
			handleErr(c, HttpError(http.StatusNotFound, "Учитель не найден в базе"))
			return
		}
		rows, err := ManyRows(`SELECT classes.id, classes.number || classes.letter AS name, schools.name AS school_name, COUNT(students.id) AS students_count FROM classes
			JOIN schools ON classes.school_id = schools.id
			JOIN subjects ON subjects.class_id = classes.id
			LEFT JOIN students ON students.class_id = classes.id
			WHERE subjects.teacher_id = $1
			GROUP BY classes.id, classes.number, classes.letter, schools.name`, ToInt64(teacherID["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	registerAdminGET(r)

	r.GET("/students/me/grades", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "student" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		rows, err := ManyRows(`SELECT grades.id, subjects.name AS subject_name, grades.grade AS value, grades.date, grades.lesson_id
			FROM grades
			JOIN subjects ON grades.subject_id = subjects.id
			JOIN students ON grades.student_id = students.id
			WHERE students.user_id = $1
			ORDER BY grades.date DESC`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/students/me/schedule", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "student" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		rows, err := ManyRows(`SELECT
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
			ORDER BY lessons.weekday, lessons.time`, ToInt64(current_user["id"]))
		if err != nil {
			if isUndefinedTable(err) {
				c.JSON(http.StatusOK, []map[string]any{})
				return
			}
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/teacher/classes/:class_id/grades", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		classID, ok := PathInt(c, "class_id")
		if !ok {
			return
		}
		rows, err := ManyRows(`SELECT
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
			ORDER BY grades.date DESC`, classID)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/teacher/schedule", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		rows, err := ManyRows(`
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
			ORDER BY lessons.weekday, lessons.time`, ToInt64(current_user["id"]))
		if err != nil {
			if isUndefinedTable(err) {
				c.JSON(http.StatusOK, []map[string]any{})
				return
			}
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})
}

func registerOther(r gin.IRouter) {
	r.PATCH("/users/me", func(c *gin.Context) {
		var payload UserUpdate
		if err := c.ShouldBindJSON(&payload); err != nil {
			handleErr(c, err)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Имя не может быть пустым"))
			return
		}
		current_user := CurrentUser(c)
		user, err := OneRow(`
			UPDATE users
			SET name = $1
			WHERE id = $2
			RETURNING *`, name, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, user)
	})

	r.POST("/users/me/avatar", func(c *gin.Context) {
		var payload AvatarUpdate
		if err := c.ShouldBindJSON(&payload); err != nil {
			handleErr(c, err)
			return
		}
		current_user := CurrentUser(c)
		avatarURL := strings.TrimSpace(payload.AvatarURL)
		if !strings.HasPrefix(avatarURL, "data:") || !strings.Contains(avatarURL, ";base64,") {
			handleErr(c, HttpError(http.StatusBadRequest, "Некорректный формат аватара"))
			return
		}
		header, encoded, found := strings.Cut(avatarURL, ",")
		if !found {
			handleErr(c, HttpError(http.StatusBadRequest, "Некорректный формат аватара"))
			return
		}
		if len(header) < 5 {
			handleErr(c, HttpError(http.StatusBadRequest, "Некорректный формат аватара"))
			return
		}
		mimeType := strings.ToLower(strings.SplitN(header[5:], ";", 2)[0])
		if !ALLOWED_AVATAR_MIME_TYPES[mimeType] {
			handleErr(c, HttpError(http.StatusBadRequest, "Поддерживаются PNG, JPG, WEBP и GIF"))
			return
		}
		fileBytes, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Не удалось декодировать изображение"))
			return
		}
		if len(fileBytes) > MAX_AVATAR_BYTES {
			handleErr(c, HttpError(http.StatusBadRequest, "Максимальный размер аватара — 2 МБ"))
			return
		}
		user, err := OneRow(`
			UPDATE users
			SET avatar_url = $1
			WHERE id = $2
			RETURNING avatar_url`, avatarURL, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"avatar_url": user["avatar_url"]})
	})

	r.DELETE("/users/me/avatar", func(c *gin.Context) {
		current_user := CurrentUser(c)
		_, err := Pool.Exec(Ctx, `
			UPDATE users
			SET avatar_url = NULL
			WHERE id = $1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.DELETE("/users/me", func(c *gin.Context) {
		current_user := CurrentUser(c)
		dbTx, err := Pool.Begin(Ctx)
		if err != nil {
			handleErr(c, err)
			return
		}
		defer dbTx.Rollback(Ctx)

		switch current_user["role"] {
		case "student":
			if _, err := dbTx.Exec(Ctx, `DELETE FROM students WHERE user_id = $1`, ToInt64(current_user["id"])); err != nil {
				handleErr(c, err)
				return
			}
		case "teacher":
			teacher, err := OneRow(`SELECT id FROM teachers WHERE user_id = $1`, ToInt64(current_user["id"]))
			if err != nil {
				handleErr(c, err)
				return
			}
			if teacher != nil {
				if _, err = dbTx.Exec(Ctx, "DELETE FROM teacher_class_subjects WHERE teacher_id = $1", ToInt64(teacher["id"])); err != nil {
					handleErr(c, err)
					return
				}
				if _, err = dbTx.Exec(Ctx, "UPDATE subjects SET teacher_id = NULL WHERE teacher_id = $1", ToInt64(teacher["id"])); err != nil {
					handleErr(c, err)
					return
				}
				if _, err = dbTx.Exec(Ctx, "DELETE FROM teachers WHERE user_id = $1", ToInt64(current_user["id"])); err != nil {
					handleErr(c, err)
					return
				}
			}
		}

		deleted, err := dbTx.Exec(Ctx, "DELETE FROM users WHERE id = $1", ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if err = dbTx.Commit(Ctx); err != nil {
			handleErr(c, err)
			return
		}
		if deleted.RowsAffected() == 0 {
			dbTx.Rollback(Ctx)
			handleErr(c, HttpError(http.StatusNotFound, "Пользователь не найден"))
			return
		}
		clearAuthCookie(c)
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.DELETE("/admin/assign/:id", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return
		}
		id, ok := PathInt(c, "id")
		if !ok {
			return
		}
		existing, err := OneRow(`SELECT id FROM teacher_class_subjects WHERE id = $1`, id)
		if err != nil {
			handleErr(c, err)
			return
		}
		if existing == nil {
			handleErr(c, HttpError(http.StatusNotFound, fmt.Sprintf("Assignment %d Not Found", id)))
			return
		}
		if _, err = Pool.Exec(Ctx, `DELETE FROM teacher_class_subjects WHERE id = $1`, id); err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.DELETE("/teacher/grades/:grade_id", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		gradeID, ok := PathInt(c, "grade_id")
		if !ok {
			return
		}
		teacher, err := OneRow("SELECT id FROM teachers WHERE user_id = $1", ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if teacher == nil {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		grade, err := OneRow(`
			SELECT g.id FROM grades g
			JOIN subjects s ON g.subject_id = s.id
			WHERE g.id = $1 AND s.teacher_id = $2`, gradeID, ToInt64(teacher["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if grade == nil {
			existing, err := OneRow("SELECT id FROM grades WHERE id = $1", gradeID)
			if err != nil {
				handleErr(c, err)
				return
			}
			if existing == nil {
				handleErr(c, HttpError(http.StatusNotFound, fmt.Sprintf("Grade %d Not Found", gradeID)))
			} else {
				handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			}
			return
		}
		if _, err = Pool.Exec(Ctx, "DELETE FROM grades WHERE id = $1", gradeID); err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.POST("/admin/users", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return
		}
		var payload AdminCreate
		if err := c.ShouldBindJSON(&payload); err != nil {
			handleErr(c, err)
			return
		}
		name := strings.TrimSpace(payload.Name)
		email := strings.ToLower(strings.TrimSpace(payload.Email))
		password := payload.Password
		if name == "" || email == "" || password == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Нужно заполнить имя, почту и пароль"))
			return
		}
		existingUser, err := OneRow("SELECT id FROM users WHERE email = $1", email)
		if err != nil {
			handleErr(c, err)
			return
		}
		if existingUser != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Email already registered"))
			return
		}
		passwordHash := HashPassword(password)
		createdUser, err := OneRow(`
			INSERT INTO users (name, email, passwd_hash, role)
			VALUES ($1, $2, $3, 'admin')
			RETURNING id, name, email, role`, name, email, passwordHash)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, createdUser)
	})

	r.POST("/admin/classes", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return
		}
		var payload ClassCreate
		if err := c.ShouldBindJSON(&payload); err != nil {
			handleErr(c, err)
			return
		}
		letter := strings.ToUpper(strings.TrimSpace(payload.Letter))
		schoolName := strings.TrimSpace(payload.SchoolName)
		if payload.Number <= 0 {
			handleErr(c, HttpError(http.StatusBadRequest, "Номер класса должен быть больше нуля"))
			return
		}
		if letter == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Буква класса обязательна"))
			return
		}
		if schoolName == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Укажите школу"))
			return
		}
		dbTx, err := Pool.Begin(Ctx)
		if err != nil {
			handleErr(c, err)
			return
		}
		defer dbTx.Rollback(Ctx)

		schoolID, err := getOrCreateSchoolID(dbTx, schoolName)
		if err != nil {
			handleErr(c, err)
			return
		}
		existingClass, err := OneRowOn(dbTx, `
			SELECT id FROM classes
			WHERE number = $1 AND UPPER(letter) = $2 AND school_id = $3`, payload.Number, letter, schoolID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if existingClass != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Такой класс уже существует"))
			return
		}
		createdClass, err := OneRowOn(dbTx, `
			INSERT INTO classes (number, letter, school_id)
			VALUES ($1, $2, $3)
			RETURNING id, number, letter, school_id`, payload.Number, letter, schoolID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if err = dbTx.Commit(Ctx); err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, createdClass)
	})

	r.POST("/admin/subjects", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return
		}
		var payload SubjectCreate
		if err := c.ShouldBindJSON(&payload); err != nil {
			handleErr(c, err)
			return
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Название предмета обязательно"))
			return
		}
		classExists, err := OneRow("SELECT id FROM classes WHERE id = $1", payload.ClassID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if classExists == nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Класс не найден"))
			return
		}
		existingSubject, err := OneRow(`
			SELECT id FROM subjects
			WHERE LOWER(name) = LOWER($1) AND class_id = $2`, name, payload.ClassID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if existingSubject != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Такой предмет уже есть у этого класса"))
			return
		}
		createdSubject, err := OneRow(`
			INSERT INTO subjects (name, class_id)
			VALUES ($1, $2)
			RETURNING id, name, class_id`, name, payload.ClassID)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, createdSubject)
	})

	r.POST("/auth/register", func(c *gin.Context) {
		var user Register
		if err := c.ShouldBindJSON(&user); err != nil {
			handleErr(c, err)
			return
		}
		existingUser, err := OneRow("SELECT id FROM users WHERE email = $1", user.Email)
		if err != nil {
			handleErr(c, err)
			return
		}
		if existingUser != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Email already registered"))
			return
		}
		passwordHash := HashPassword(user.Password)
		if user.Role != "student" && user.Role != "teacher" {
			handleErr(c, HttpError(http.StatusBadRequest, "Incorrect role"))
			return
		}
		if user.Role == "student" {
			if user.ClassID == nil {
				handleErr(c, HttpError(http.StatusBadRequest, "Class ID is required for this role"))
				return
			}
			classExists, err := OneRow("SELECT 1 FROM classes WHERE id = $1", *user.ClassID)
			if err != nil {
				handleErr(c, err)
				return
			}
			if classExists == nil {
				handleErr(c, HttpError(http.StatusBadRequest, "Class doesn't exist"))
				return
			}
		}
		idRecord, err := OneRow(`
			INSERT INTO users (name, email, passwd_hash, role)
			VALUES ($1, $2, $3, $4)
			RETURNING id`, user.Name, user.Email, passwordHash, user.Role)
		if err != nil {
			handleErr(c, err)
			return
		}
		userID := ToInt64(idRecord["id"])
		if user.Role == "student" {
			_, err = Pool.Exec(Ctx, "INSERT INTO students (user_id, class_id) VALUES ($1, $2)", userID, *user.ClassID)
		} else if user.Role == "teacher" {
			_, err = Pool.Exec(Ctx, "INSERT INTO teachers (user_id) VALUES ($1)", userID)
		}
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.POST("/auth/login", func(c *gin.Context) {
		var user Login
		if err := c.ShouldBindJSON(&user); err != nil {
			handleErr(c, err)
			return
		}
		dbUser, err := OneRow("SELECT * FROM users WHERE email = $1", user.Email)
		if err != nil {
			handleErr(c, err)
			return
		}
		if dbUser == nil || !VerifyPassword(user.Password, ifaceToString(dbUser["passwd_hash"])) {
			handleErr(c, HttpError(http.StatusUnauthorized, "Invalid email or password"))
			return
		}
		token, err := CreateAccessToken(map[string]any{"user_id": ToInt64(dbUser["id"])})
		if err != nil {
			handleErr(c, err)
			return
		}
		setAuthCookie(c, token)
		c.JSON(http.StatusOK, gin.H{"access_token": token})
	})

	r.POST("/teacher/grades", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden :|"))
			return
		}
		var gradeData map[string]any
		if err := c.ShouldBindJSON(&gradeData); err != nil {
			handleErr(c, err)
			return
		}
		studentID := jsonAnyToInt64(gradeData["student_id"])
		subjectID := jsonAnyToInt64(gradeData["subject_id"])
		value := gradeData["value"]
		dateStr := ifaceToString(gradeData["date"])
		if studentID == 0 || value == nil || dateStr == "" {
			handleErr(c, HttpError(http.StatusBadRequest, "Missing required grade fields"))
			return
		}
		teacherIDRow, err := OneRow(`SELECT id FROM teachers
			WHERE user_id = $1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if teacherIDRow == nil {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden. You aren't a teacher"))
			return
		}
		teacherID := ToInt64(teacherIDRow["id"])
		var subjectTeacherClassID map[string]any
		if subjectID == 0 {
			studentClassID, err := OneRow(`SELECT class_id FROM students WHERE id = $1`, studentID)
			if err != nil {
				handleErr(c, err)
				return
			}
			if studentClassID == nil {
				handleErr(c, HttpError(http.StatusNotFound, "Not Found"))
				return
			}
			row, err := OneRow(`SELECT id, class_id, teacher_id FROM subjects
				WHERE class_id = $1 AND teacher_id = $2
				ORDER BY id
				LIMIT 1`, ToInt64(studentClassID["class_id"]), teacherID)
			if err != nil {
				handleErr(c, err)
				return
			}
			if row != nil {
				subjectID = ToInt64(row["id"])
				subjectTeacherClassID = row
			}
		} else {
			row, err := OneRow(`SELECT id, class_id, teacher_id FROM subjects WHERE id = $1`, subjectID)
			if err != nil {
				handleErr(c, err)
				return
			}
			subjectTeacherClassID = row
		}
		if subjectTeacherClassID == nil {
			handleErr(c, HttpError(http.StatusNotFound, "Subject not found for this teacher and class"))
			return
		}
		if teacherID != ToInt64(subjectTeacherClassID["teacher_id"]) {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden :|"))
			return
		}
		studentClassID, err := OneRow(`SELECT class_id FROM students WHERE id = $1`, studentID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if studentClassID == nil {
			handleErr(c, HttpError(http.StatusNotFound, "Not Found"))
			return
		}
		if ToInt64(subjectTeacherClassID["class_id"]) != ToInt64(studentClassID["class_id"]) {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden :|"))
			return
		}
		insertDate, parseErr := time.Parse("2006-01-02", dateStr)
		if parseErr != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Missing required grade fields"))
			return
		}
		if _, err = Pool.Exec(Ctx, `
		INSERT INTO grades (student_id, subject_id, grade, date)
		VALUES ($1, $2, $3, $4)`, studentID, subjectID, value, insertDate); err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.POST("/select_class", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "teacher" {
			handleErr(c, HttpError(http.StatusForbidden, "Forbidden"))
			return
		}
		classID, ok := QueryInt(c, "class_id")
		if !ok {
			return
		}
		teacherClassID, err := OneRow(`SELECT class_id FROM teachers
		WHERE user_id = $1`, ToInt64(current_user["id"]))
		if err != nil {
			handleErr(c, err)
			return
		}
		if teacherClassID == nil {
			handleErr(c, HttpError(http.StatusInternalServerError, "Internal Server Error"))
			return
		}
		if teacherClassID["class_id"] != nil {
			handleErr(c, HttpError(http.StatusBadRequest, "У вас уже есть класс"))
			return
		}
		classRow, err := OneRow(`SELECT 1 FROM classes WHERE class_id = $1`, classID)
		if err != nil {
			handleErr(c, err)
			return
		}
		if classRow == nil {
			handleErr(c, HttpError(http.StatusBadRequest, "Class doesn't exist"))
			return
		}
		if _, err = Pool.Exec(Ctx, `UPDATE teachers SET class_id = $1 WHERE user_id = $2`, classID, ToInt64(current_user["id"])); err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successful"})
	})

	r.POST("/admin/assign", func(c *gin.Context) {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return
		}
		var assign Assign
		if err := c.ShouldBindJSON(&assign); err != nil {
			handleErr(c, err)
			return
		}
		assignment, err := OneRow(`
			INSERT INTO teacher_class_subjects (teacher_id, class_id, subject_id)
			VALUES ($1, $2, $3)
			RETURNING id`, assign.TeacherID, assign.ClassID, assign.SubjectID)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, assignment["id"])
	})
}

func registerAdminGET(r gin.IRouter) {
	adminOnly := func(c *gin.Context) bool {
		current_user := CurrentUser(c)
		if current_user["role"] != "admin" {
			handleErr(c, HttpError(http.StatusForbidden, "Admin Only Endpoint"))
			return false
		}
		return true
	}

	r.GET("/admin/assignments", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT
				tcs.id,
				users.name as teacher,
				classes.number || classes.letter as class,
				subjects.name as subject
			FROM teacher_class_subjects tcs
			JOIN teachers ON tcs.teacher_id = teachers.id
			JOIN users ON teachers.user_id = users.id
			JOIN classes ON tcs.class_id = classes.id
			JOIN subjects ON tcs.subject_id = subjects.id`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/classes", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT classes.id, classes.number || classes.letter AS name, COUNT(students.id) AS students_count FROM classes
			JOIN schools ON schools.id = classes.school_id
			LEFT JOIN students ON students.class_id = classes.id
			GROUP BY classes.id, classes.number, classes.letter, schools.name`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/schools", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT id, name
			FROM schools
			ORDER BY name`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/analytics", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT
				(SELECT COUNT(*) FROM users) AS total_users,
				(SELECT COUNT(*) FROM users WHERE role = 'student') AS total_students,
				(SELECT COUNT(*) FROM users WHERE role = 'teacher') AS total_teachers,
				(SELECT COUNT(*) FROM users WHERE role = 'admin') AS total_admins,
				(SELECT COUNT(*) FROM classes) AS total_classes,
				(SELECT COALESCE(AVG(grades.grade), 0) FROM grades) AS avg_grade`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/subjects", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT subjects.id, subjects.name, classes.number || classes.letter AS class FROM subjects
			JOIN classes ON subjects.class_id = classes.id`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/teachers", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT teachers.id, users.name FROM teachers
			JOIN users ON teachers.user_id = users.id`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})

	r.GET("/admin/admins", func(c *gin.Context) {
		if !adminOnly(c) {
			return
		}
		rows, err := ManyRows(`
			SELECT id, name, email
			FROM users
			WHERE role = 'admin'
			ORDER BY name`)
		if err != nil {
			handleErr(c, err)
			return
		}
		c.JSON(http.StatusOK, rows)
	})
}

func QueryInt(c *gin.Context, key string) (int64, bool) {
	raw := c.Query(key)
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		c.JSON(422, gin.H{"detail": []gin.H{{
			"type": "int_parsing",
			"loc":  []string{"query", key},
			"msg":  "Input should be a valid integer, unable to parse string as an integer",
		}}})
		return 0, false
	}
	return v, true
}

func PathInt(c *gin.Context, key string) (int64, bool) {
	v, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		c.JSON(422, gin.H{"detail": []gin.H{{
			"type": "int_parsing",
			"loc":  []string{"path", key},
			"msg":  "Input should be a valid integer, unable to parse string as an integer",
		}}})
		return 0, false
	}
	return v, true
}

func QueryOptionalInt64(c *gin.Context, key string) *int64 {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return &v
	}
	return nil
}

func QueryOptionalBool(c *gin.Context, key string) bool {
	switch strings.ToLower(c.Query(key)) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

func jsonAnyToInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case string:
		if parsed, err := strconv.ParseInt(n, 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func ifaceToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// --- deploy additions: serve web/ statics + /api/* alias ---

var Router *gin.Engine

const webDir = "web"

func webIndexHTTP(w http.ResponseWriter, r *http.Request) bool {
	idx := filepath.Join(webDir, "index.html")
	if _, err := os.Stat(idx); err != nil {
		return false
	}
	http.ServeFile(w, r, idx)
	return true
}

func notFoundHandler(c *gin.Context) {
	path := c.Request.URL.Path
	// frontend always calls /api/..., backend routes are registered without the prefix
	if strings.HasPrefix(path, "/api") && !c.GetBool("apiAliased") {
		c.Request.URL.Path = strings.TrimPrefix(path, "/api")
		c.Set("apiAliased", true)
		Router.HandleContext(c)
		return
	}
	clean := filepath.Clean("/" + path)
	full := filepath.Join(webDir, clean)
	if !strings.HasPrefix(full, filepath.Clean(webDir)+string(os.PathSeparator)) && !(clean == "/" || clean == ".") {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not Found :(", "path": path, "message": fmt.Sprintf("Route '%s' not found", path)})
		return
	}
	if clean == "/" || strings.HasSuffix(clean, "/") {
		if webIndexHTTP(c.Writer, c.Request) { return }
	} else if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
		http.ServeFile(c.Writer, c.Request, full)
		return
	}
	if fi, err := os.Stat(full + ".html"); err == nil && !fi.IsDir() {
		http.ServeFile(c.Writer, c.Request, full+".html")
		return
	}
	if webIndexHTTP(c.Writer, c.Request) { return }
	if fi, err := os.Stat(filepath.Join(webDir, "404.html")); err == nil && !fi.IsDir() {
		http.ServeFile(c.Writer, c.Request, filepath.Join(webDir, "404.html"))
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Not Found :(", "path": path, "message": fmt.Sprintf("Route '%s' not found", path)})
}
