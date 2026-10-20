package main

import (
	"log"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) string {
	passwordBytes := []byte(password)
	if len(passwordBytes) > 72 {
		passwordBytes = passwordBytes[:72]
	}
	hashed, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Ошибка хэширования пароля: %v", err)
	}
	return string(hashed)
}

func VerifyPassword(password, hashed string) bool {
	passwordBytes := []byte(password)
	if len(passwordBytes) > 72 {
		passwordBytes = passwordBytes[:72]
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashed), passwordBytes)
	if err != nil {
		if err == bcrypt.ErrHashTooShort {
			log.Printf("Ошибка верификации: %v", err)
		}
		return false
	}
	return true
}
