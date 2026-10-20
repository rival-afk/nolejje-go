from typing import Optional
from pydantic import BaseModel
from datetime import date

class Register(BaseModel):
  name: str
  email: str
  password: str
  role: str
  class_id: Optional[int]

class Login(BaseModel):
  email: str
  password: str

class GradePost(BaseModel):
  student_id: int
  subject_id: int
  grade: int
  date: date

class Assign(BaseModel):
  teacher_id: int
  class_id: int
  subject_id: int

class UserUpdate(BaseModel):
  name: str

class AvatarUpdate(BaseModel):
  avatar_url: str

class AdminCreate(BaseModel):
  name: str
  email: str
  password: str

class ClassCreate(BaseModel):
  number: int
  letter: str
  school_name: str

class SubjectCreate(BaseModel):
  name: str
  class_id: int
