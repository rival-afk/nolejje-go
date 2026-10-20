package main

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

var (
	ALGORITHM    = jwt.SigningMethodHS256
	TOKEN_EXPIRE = 1440 * time.Minute
	SECRET_KEY   string
)

func init() {
	godotenv.Load()
	SECRET_KEY = os.Getenv("jwt_secret_key")
}

func CreateAccessToken(data map[string]any) (string, error) {
	claims := jwt.MapClaims{}
	for k, v := range data {
		claims[k] = v
	}
	claims["exp"] = time.Now().Add(TOKEN_EXPIRE).Unix()
	token := jwt.NewWithClaims(ALGORITHM, claims)
	return token.SignedString([]byte(SECRET_KEY))
}
