package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashAndSaltPassword ...
func HashAndSaltPassword(password string) (string, error) {
	hashedPasswordBytes, err :=
		bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("error hashing password: %v", err)
	}
	hashedPassword := string(hashedPasswordBytes)
	return hashedPassword, nil
}

// ComparePasswords ...
func ComparePasswords(plainPassword string, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	if err != nil {
		return false
	}
	return true
}
