package healthcheck

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gorm.io/gorm"
)

type Controller struct {
	db  *gorm.DB
	cfg *pkg.Config
}

func (hc *Controller) HandleLiveness(c *fiber.Ctx) error {
	c.Response().SetBodyString(fmt.Sprintf(`%s OK`, hc.cfg.App.Name))
	return nil
}

func (hc *Controller) HandleReadiness(c *fiber.Ctx) error {
	// instance := hc.cfg.App.Instance
	//db check
	// wRep := repositories.NewWorkersInstallsRepository(hc.db)
	// ctx := context.Background()
	// _, err := wRep.FindLast(ctx)
	// if err != nil {
	// 	msg := fmt.Sprintf(`%s instance "%s" database error: %v`, hc.cfg.App.Name, instance, err)
	// 	return &fiber.Error{Code: fiber.StatusInternalServerError, Message: msg}
	// }
	c.Response().SetBodyString(fmt.Sprintf(`%s OK`, hc.cfg.App.Name))
	return nil
}

func RegisterHealthCheckController(router fiber.Router, container *pkg.Container) {
	ctrl := NewHealthCheckController(container)
	router.Get("/healthcheck/liveness", ctrl.HandleLiveness)
	router.Get("/healthcheck/readiness", ctrl.HandleReadiness)
}

func NewHealthCheckController(container *pkg.Container) *Controller {
	return &Controller{
		db:  container.DB,
		cfg: container.Config,
	}
}
