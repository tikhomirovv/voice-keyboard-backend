package controllers

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

func (ac *PublicController) InitiateYandexAuthAction(c *fiber.Ctx) error {
	// Получаем state из параметров запроса
	state := c.Query("state")
	// Если state не существует, создаем новый
	if state == "" {
		state = uuid.New().String()
	}
	// Сохраняем state в cookie для последующей проверки
	cookie := new(fiber.Cookie)
	cookie.Name = "oauth_state"
	cookie.Value = state
	cookie.Expires = time.Now().Add(15 * time.Minute)
	cookie.HTTPOnly = true
	cookie.Secure = true // для HTTPS
	c.Cookie(cookie)

	// Получаем URL для авторизации
	authURL := ac.yo.GetAuthorizationURL(state)

	return c.Redirect(authURL)
}

func (ac *PublicController) YandexCallbackAction(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		ac.log.Error("PublicController.YandexCallbackAction: No code provided in Yandex callback")
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "No authorization code provided",
		}
	}

	// Проверяем state для защиты от CSRF
	expectedState := c.Cookies("oauth_state")
	if state == "" || state != expectedState {
		ac.log.Error("PublicController.YandexCallbackAction: Invalid state parameter",
			"expected", expectedState,
			"received", state,
		)
		return &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid state parameter",
		}
	}

	socialAuthDTO, err := ac.yo.GetSocialAuthByCode(c.UserContext(), code)
	if err != nil {
		ac.log.Error("PublicController.YandexCallbackAction: Failed to get user info", "error", err)
		return &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to get user info",
		}
	}

	// Авторизуем пользователя
	userToken, err := ac.as.SignInWithSocial(c.UserContext(), socialAuthDTO)
	if err != nil {
		ac.log.Error("PublicController.YandexCallbackAction: Failed to sign in with social", "error", err)
		return &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to complete authentication",
		}
	}

	return c.JSON(pkg.NewResponseBody(userToken))
}

func RegisterPublicController(router fiber.Router, container *pkg.Container) {
	ctrl := NewPublicController(container)
	// yandex auth
	router.Get("/auth/yandex/login", ctrl.InitiateYandexAuthAction)
	router.Get("/auth/yandex/callback", ctrl.YandexCallbackAction)
}

func NewPublicController(cnt *pkg.Container) *PublicController {
	uti, _ := pkg.NewValidatorTranslator(cnt.Validator)
	return &PublicController{
		vl:  cnt.Validator,
		ut:  uti,
		log: cnt.Logger,
		as:  cnt.AuthService,
		yo:  cnt.YandexOAuthService,
	}
}

type PublicController struct {
	vl  *validator.Validate
	ut  ut.Translator
	log logger.Logger
	as  interfaces.AuthServiceInterface
	yo  interfaces.YandexOAuthServiceInterface
}
