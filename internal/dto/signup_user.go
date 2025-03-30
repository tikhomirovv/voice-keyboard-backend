package dto

import "net"

type SignUpUserDTO struct {
	Email          string `json:"email" validate:"required,email"`
	Password       string `json:"password" validate:"required,min=6"`
	PasswordRepeat string `json:"password_repeat" validate:"required,min=6,eqfield=Password"`
	IP             net.IP
}
