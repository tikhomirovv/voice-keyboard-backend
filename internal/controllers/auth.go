package controllers

import (
	"fmt"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/auth"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

type AuthController struct {
	vl  *validator.Validate
	ut  ut.Translator
	log logger.Logger
	as  interfaces.AuthServiceInterface
	yo  interfaces.YandexOAuthServiceInterface
}

func (ac *AuthController) RefreshTokensPairAction(c *fiber.Ctx) error {
	refreshDTO := &dto.RefreshTokensPairDTO{}

	if err := c.BodyParser(refreshDTO); err != nil {
		return &fiber.Error{Code: fiber.StatusBadRequest, Message: err.Error()}
	}
	if err := ac.vl.Struct(refreshDTO); err != nil {
		errs := err.(validator.ValidationErrors)
		return &fiber.Error{Code: fiber.StatusBadRequest, Message: errs[0].Translate(ac.ut)}
	}
	token, err := ac.as.RefreshTokensPair(c.UserContext(), refreshDTO)
	if err != nil {
		return &fiber.Error{Code: fiber.StatusBadRequest, Message: err.Error()}
	}
	body := pkg.NewResponseBody(token)
	if err = c.JSON(body); err != nil {
		return &fiber.Error{Code: fiber.StatusInternalServerError, Message: err.Error()}
	}
	return nil
}

func (ac *AuthController) SignOutAction(c *fiber.Ctx) error {
	user, err := auth.ExtractUser(c.Locals("user"))
	if err != nil {
		return &fiber.Error{Code: fiber.StatusUnauthorized, Message: err.Error()}
	}
	fmt.Println(user)
	return nil
}

func (ac *AuthController) SignInWithSocialAction(c *fiber.Ctx) error {
	socialAuthDTO := &dto.SocialAuthDTO{}

	if err := c.BodyParser(socialAuthDTO); err != nil {
		ac.log.Error("Failed to parse social auth request", "error", err)
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid request body",
		}
	}

	if err := ac.vl.Struct(socialAuthDTO); err != nil {
		errs := err.(validator.ValidationErrors)
		ac.log.Error("Social auth validation failed", "error", errs)
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: errs[0].Translate(ac.ut),
		}
	}

	token, err := ac.as.SignInWithSocial(c.Context(), socialAuthDTO)
	if err != nil {
		ac.log.Error("Social auth failed", "error", err)
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
		}
	}

	return c.JSON(pkg.NewResponseBody(token))
}

func RegisterAuthController(router fiber.Router, container *pkg.Container) {
	ctrl := NewAuthController(container)
	router.Post("/refresh", ctrl.RefreshTokensPairAction)
	router.Put("/signout", ctrl.SignOutAction)
}

func NewAuthController(cnt *pkg.Container) *AuthController {
	uti, _ := pkg.NewValidatorTranslator(cnt.Validator)
	return &AuthController{
		vl:  cnt.Validator,
		ut:  uti,
		log: cnt.Logger,
		as:  cnt.AuthService,
		yo:  cnt.YandexOAuthService,
	}
}
