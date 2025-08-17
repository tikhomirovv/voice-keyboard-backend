package controllers

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/internal/middleware"
	"gitlab.com/voice-keyboard/backend-go/internal/services/transcribe"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// Размеры буферов для WebSocket соединения
const (
	WebSocketReadBufferSize  = 1024
	WebSocketWriteBufferSize = 1024
)

// WebSocketController обрабатывает WebSocket соединения
type WebSocketController struct {
	logger            logger.Logger
	validator         *validator.Validate
	authService       interfaces.AuthServiceInterface
	transcribeHandler *transcribe.TranscribeWSHandler
}

// HandleWebSocket обрабатывает WebSocket соединение для аудио
func (wc *WebSocketController) HandleWebSocket(c *websocket.Conn) {
	// Делегируем обработку TranscribeWSHandler'у
	wc.transcribeHandler.HandleWebSocket(c)
}

// HandleMonitorPage отображает страницу мониторинга WebSocket соединений
func (wc *WebSocketController) HandleMonitorPage(c *fiber.Ctx) error {
	// Рендерим шаблон monitor.html
	return c.Render("monitor", fiber.Map{})
}

// HandleMonitorData возвращает данные мониторинга WebSocket соединений
func (wc *WebSocketController) HandleMonitorData(c *fiber.Ctx) error {
	data := fiber.Map{
		"active_sessions": wc.transcribeHandler.GetSessionsCount(),
		"active_users":    wc.transcribeHandler.GetUsersCount(),
		"sessions":        wc.transcribeHandler.GetSessionsInfo(),
	}

	return c.JSON(data)
}

// RegisterWebSocketController регистрирует маршруты для WebSocketController
func RegisterWebSocketController(router fiber.Router, container *pkg.Container) {
	ctrl := NewWebSocketController(container)

	// WebSocket middleware для проверки авторизации
	wsMiddleware := func(c *fiber.Ctx) error {
		// Проверка авторизации по заголовку Authorization
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authorization required",
			})
		}

		// Обычно заголовок имеет формат "Bearer <token>"
		tokenString := authHeader[7:] // Удаляем "Bearer "
		userID, err := ctrl.authService.ValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid token: %v", err),
			})
		}

		// Сохраняем userID в контексте для использования в WebSocket обработчике
		c.Locals("userID", userID)
		return c.Next()
	}

	// WebSocket обработчик для аудио
	wsHandler := websocket.New(ctrl.HandleWebSocket, websocket.Config{
		ReadBufferSize:  WebSocketReadBufferSize,
		WriteBufferSize: WebSocketWriteBufferSize,
	})

	// Логируем текущую конфигурацию Basic Auth
	container.Logger.Info("Basic Auth configuration",
		"username", container.Config.BasicAuth.Username,
		"password_set", container.Config.BasicAuth.Password != "")

	// Middleware для Basic Authentication страниц мониторинга
	basicAuthMiddleware := middleware.NewBasicAuthMiddleware(container.Config, container.Logger)

	// Регистрация маршрутов
	router.Get("/", wsMiddleware, wsHandler)
	router.Get("/monitor", basicAuthMiddleware, ctrl.HandleMonitorPage)
	router.Get("/monitor/data", basicAuthMiddleware, ctrl.HandleMonitorData)
}

// NewWebSocketController создает новый экземпляр WebSocketController
func NewWebSocketController(cnt *pkg.Container) *WebSocketController {
	// Создаем процессор для обработки WebSocket сообщений
	processor := transcribe.NewRealtimeWebSocketService(
		cnt.Logger,
		cnt.RealtimeTranscriberService,
		cnt.OpenAITextGenerationService,
	)

	// Создаем обработчик WebSocket соединений
	transcribeHandler := transcribe.NewTranscribeWSHandler(cnt, processor)

	return &WebSocketController{
		logger:            cnt.Logger,
		validator:         cnt.Validator,
		authService:       cnt.AuthService,
		transcribeHandler: transcribeHandler,
	}
}
