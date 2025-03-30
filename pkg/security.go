package pkg

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordGenerator interface {
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) error
}

type PasswordGeneratorBCrypt struct {
	Cost int
}

func (pg *PasswordGeneratorBCrypt) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), pg.Cost)
	return string(bytes), err
}

func (pg *PasswordGeneratorBCrypt) CheckPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
