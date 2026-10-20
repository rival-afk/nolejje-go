package main

type Register struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	ClassID  *int64 `json:"class_id"`
}

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GradePost struct {
	StudentID int64  `json:"student_id"`
	SubjectID int64  `json:"subject_id"`
	Grade     int    `json:"grade"`
	Date      string `json:"date"`
}

type Assign struct {
	TeacherID int64 `json:"teacher_id"`
	ClassID   int64 `json:"class_id"`
	SubjectID int64 `json:"subject_id"`
}

type UserUpdate struct {
	Name string `json:"name"`
}

type AvatarUpdate struct {
	AvatarURL string `json:"avatar_url"`
}

type AdminCreate struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ClassCreate struct {
	Number     int64  `json:"number"`
	Letter     string `json:"letter"`
	SchoolName string `json:"school_name"`
}

type SubjectCreate struct {
	Name    string `json:"name"`
	ClassID int64  `json:"class_id"`
}
