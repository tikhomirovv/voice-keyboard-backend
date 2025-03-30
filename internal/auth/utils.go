package auth

import (
	"fmt"

	"github.com/jaswdr/faker/v2"
	"gitlab.com/voice-keyboard/backend-go/pkg"
)

func GeneratePasswordHash(password string) (string, error) {
	gen := &pkg.PasswordGeneratorBCrypt{Cost: 10}
	return gen.HashPassword(password)
}

func GenerateEmailConfirmationCode() string {
	fkr := faker.New()
	return fmt.Sprintf("%d", fkr.RandomNumber(5))
}
