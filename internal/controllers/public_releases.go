package controllers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
)

// GetReleasesAction обрабатывает запрос на получение информации о доступных обновлениях
func (ac *PublicController) GetUpdaterAction(c *fiber.Ctx) error {
	env := c.Params("env")
	target := c.Params("target")
	arch := c.Params("arch")
	currentVersion := c.Params("current_version")

	// example
	// {"arch": String("x86_64"), "current_version": String("0.0.1"), "env": String("local"), "target": String("windows")}

	ac.log.Info(fmt.Sprintf("GetUpdaterAction: env=%s, target=%s, arch=%s, currentVersion=%s",
		env, target, arch, currentVersion))

	releases, err := ac.rs.GetReleases(context.Background(), &dto.ReleasesOptions{
		Env:            env,
		Target:         target,
		Arch:           arch,
		CurrentVersion: currentVersion,
	})

	if err != nil {
		ac.log.Error(fmt.Sprintf("Error in GetUpdaterAction: %s", err.Error()))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Если обновления не требуются или файлы отсутствуют, возвращаем 204 No Content
	if len(releases) == 0 {
		ac.log.Info("No updates available")
		return c.SendStatus(fiber.StatusNoContent)
	}

	// Если необходимо обновление, возвращаем 200 OK и информацию о релизе
	ac.log.Info(fmt.Sprintf("Update available: version=%s", releases[0].Version))
	return c.Status(fiber.StatusOK).JSON(releases[0])
}

// GetReleaseFileAction обрабатывает запросы на скачивание файлов релизов
func (ac *PublicController) GetReleaseFileAction(c *fiber.Ctx) error {
	// Получаем относительный путь к файлу из URL
	relPath := c.Params("*")
	if relPath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Path not specified",
		})
	}

	// Формируем полный путь к файлу
	filePath := filepath.Join(ac.cfg.Releases.Path, relPath)
	ac.log.Info(fmt.Sprintf("Request for release file: %s", filePath))

	// Проверяем, существует ли файл
	if err := c.SendFile(filePath); err != nil {
		ac.log.Error(fmt.Sprintf("Failed to send file: %s, error: %s", filePath, err.Error()))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found",
		})
	}

	return nil
}
