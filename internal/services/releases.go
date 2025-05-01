package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

type ReleasesService struct {
	cfg *pkg.Config
	log logger.Logger
}

func NewReleasesService(cfg *pkg.Config, log logger.Logger) *ReleasesService {
	return &ReleasesService{
		cfg: cfg,
		log: log,
	}
}

// GetReleases возвращает информацию о доступных обновлениях
func (s *ReleasesService) GetReleases(ctx context.Context, options *dto.ReleasesOptions) ([]*dto.Release, error) {
	s.log.Info(fmt.Sprintf("Checking for updates: env=%s, target=%s, arch=%s, currentVersion=%s",
		options.Env, options.Target, options.Arch, options.CurrentVersion))

	// Проверяем нужно ли обновление
	needsUpdate, err := s.needsUpdate(options.CurrentVersion, s.cfg.Releases.ActualVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to check if update needed: %w", err)
	}

	if !needsUpdate {
		s.log.Info(fmt.Sprintf("No update needed for version %s", options.CurrentVersion))
		return nil, nil
	}

	// Получаем информацию о релизе
	release, err := s.getRelease(options.Env, options.Target, options.Arch, s.cfg.Releases.ActualVersion)
	if err != nil {
		return nil, err
	}

	if release == nil {
		return nil, nil
	}

	return []*dto.Release{release}, nil
}

// needsUpdate проверяет, нужно ли обновление
func (s *ReleasesService) needsUpdate(currentVersion, actualVersion string) (bool, error) {
	// Удаляем префикс "v" из версий, если он есть
	currentVersion = strings.TrimPrefix(currentVersion, "v")
	actualVersion = strings.TrimPrefix(actualVersion, "v")

	// Проверяем, что версии отличаются
	return currentVersion != actualVersion, nil
}

// getRelease получает информацию о релизе
func (s *ReleasesService) getRelease(env, target, arch, version string) (*dto.Release, error) {
	// Формируем путь к директории релиза
	releasePath := filepath.Join(s.cfg.Releases.Path, env, target, arch, version)
	s.log.Info(fmt.Sprintf("Looking for release in directory: %s", releasePath))

	// Проверяем существование директории
	if _, err := os.Stat(releasePath); os.IsNotExist(err) {
		s.log.Info(fmt.Sprintf("Release directory not found: %s", releasePath))
		return nil, nil
	}

	// Поиск файла программы
	programFile, err := s.findProgramFile(releasePath, target)
	s.log.Info(fmt.Sprintf("Program file: %s", programFile))
	if err != nil {
		return nil, fmt.Errorf("failed to find program file: %w", err)
	}

	if programFile == "" {
		s.log.Info(fmt.Sprintf("Program file not found in directory: %s", releasePath))
		return nil, nil
	}

	// Поиск файла сигнатуры
	signatureFile := programFile + ".sig"
	if _, err := os.Stat(signatureFile); os.IsNotExist(err) {
		s.log.Info(fmt.Sprintf("Signature file not found: %s", signatureFile))
		return nil, nil
	}

	// Получение содержимого файла сигнатуры
	signatureBytes, err := os.ReadFile(signatureFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read signature file: %w", err)
	}

	// Получение информации о файле для определения даты создания/модификации
	fileInfo, err := os.Stat(programFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Формирование URL для скачивания
	releaseURL := fmt.Sprintf("%s/%s/%s/%s/%s/%s/%s",
		s.cfg.App.BaseUrl,
		"releases",
		env,
		target,
		arch,
		version,
		filepath.Base(programFile))

	// Создание и заполнение объекта Release
	release := &dto.Release{
		Version:   version,
		URL:       releaseURL,
		Signature: string(signatureBytes),
		PubDate:   fileInfo.ModTime().Format(time.RFC3339),
	}

	// Добавление примечаний к релизу, если они есть
	notes, err := s.getReleaseNotes(releasePath)
	if err != nil {
		s.log.Warn(fmt.Sprintf("Failed to get release notes: %v", err))
	} else if notes != "" {
		release.Notes = notes
	}

	return release, nil
}

// findProgramFile находит файл программы в зависимости от целевой платформы
func (s *ReleasesService) findProgramFile(dirPath, target string) (string, error) {
	var fileExtRegex *regexp.Regexp

	switch target {
	case "windows":
		fileExtRegex = regexp.MustCompile(`\.(msi|exe)$`)
	// Можно добавить обработку для других платформ в будущем
	// case "macos":
	//     fileExtRegex = regexp.MustCompile(`\.dmg$`)
	// case "linux":
	//     fileExtRegex = regexp.MustCompile(`\.AppImage$`)
	default:
		return "", fmt.Errorf("unsupported target platform: %s", target)
	}

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && fileExtRegex.MatchString(file.Name()) {
			return filepath.Join(dirPath, file.Name()), nil
		}
	}

	return "", nil
}

// getReleaseNotes читает содержимое файла с примечаниями к релизу
func (s *ReleasesService) getReleaseNotes(dirPath string) (string, error) {
	notesPath := filepath.Join(dirPath, "NOTES.md")

	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return "", nil
	}

	notesBytes, err := os.ReadFile(notesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read notes file: %w", err)
	}

	return string(notesBytes), nil
}
