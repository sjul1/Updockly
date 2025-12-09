package auth

import (
	"golang.org/x/crypto/bcrypt"
)

func hashSecret(value string) string {
	bytes, _ := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(bytes)
}

func checkPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
