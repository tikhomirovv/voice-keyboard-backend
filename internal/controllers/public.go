package controllers

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

func RegisterPublicController(router fiber.Router, container *pkg.Container) {
	ctrl := NewPublicController(container)

	router.Get("/", func(c *fiber.Ctx) error {
		cfg := container.Config
		appName := cfg.App.Name
		return c.SendString(appName)
	})

	// yandex auth
	router.Get("/auth/yandex/login", ctrl.InitiateYandexAuthAction)
	router.Get("/auth/yandex/connect", ctrl.YandexCallbackAction)
	router.Get("/updater/:env/:target/:arch/:current_version", ctrl.GetUpdaterAction)

	// Добавляем роут для скачивания файлов релизов
	router.Get("/releases/*", ctrl.GetReleaseFileAction)
}

func NewPublicController(cnt *pkg.Container) *PublicController {
	uti, _ := pkg.NewValidatorTranslator(cnt.Validator)
	return &PublicController{
		vl:  cnt.Validator,
		ut:  uti,
		log: cnt.Logger,
		as:  cnt.AuthService,
		yo:  cnt.YandexOAuthService,
		rs:  cnt.ReleasesService,
		cfg: cnt.Config,
	}
}

type PublicController struct {
	vl  *validator.Validate
	ut  ut.Translator
	log logger.Logger
	as  interfaces.AuthServiceInterface
	yo  interfaces.YandexOAuthServiceInterface
	rs  interfaces.ReleasesServiceInterface
	cfg *pkg.Config
}
