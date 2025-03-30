package auth

import (
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID uint64 `json:"id"`
}

func ExtractUser(auth any) (*User, error) {
	user, ok := auth.(*jwt.Token)
	if !ok {
		return nil, fmt.Errorf("jwt token cast error")
	}
	claims, ok := user.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("jwt claims cast error")
	}
	usr, ok := claims["user"]
	if !ok {
		return nil, fmt.Errorf("auth user not found")
	}
	bt, err := json.Marshal(usr)
	if err != nil {
		return nil, err
	}
	var u User
	err = json.Unmarshal(bt, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
