package controllers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	// Рендерим страницу с токенами
	return c.Render("auth_success", fiber.Map{
		"AccessToken":  userToken.AccessToken,
		"RefreshToken": userToken.RefreshToken,
		"AppScheme":    ac.cfg.DesktopApp.Scheme,
		"AppPath":      ac.cfg.DesktopApp.AuthPath,
	})

	// return c.JSON(pkg.NewResponseBody(userToken))
}
