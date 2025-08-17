package controllers

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
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
	// HTML страница для мониторинга с таблицей и автообновлением
	html := `
	<!DOCTYPE html>
	<html lang="en">
	<head>
	  <meta charset="UTF-8">
	  <title>WebSocket Monitor</title>
	  <style>
	    body { font-family: sans-serif; margin: 2em; }
	    table { border-collapse: collapse; min-width: 800px; margin-bottom: 20px; }
	    th, td { border: 1px solid #ccc; padding: 6px 12px; text-align: left; }
	    th { background: #f0f0f0; }
	    caption { font-size: 1.2em; margin-bottom: 0.5em; font-weight: bold; }
	    .stats { margin-bottom: 20px; }
	    .stats span { font-weight: bold; margin-right: 20px; }
	    .refresh { margin-bottom: 20px; }
	    .timestamp { margin-top: 20px; color: #666; font-size: 0.9em; }
	    .empty-row { text-align: center; color: #666; }
	  </style>
	</head>
	<body>
	  <h1>WebSocket Sessions Monitor</h1>

	  <div class="stats">
	    <span>Active Sessions: <span id="sessions-count">0</span></span>
	    <span>Active Users: <span id="users-count">0</span></span>
	  </div>

	  <div class="refresh">
	    <button onclick="fetchData()">Refresh Now</button>
	    <label>
	      <input type="checkbox" id="auto-refresh" checked>
	      Auto-refresh every
	      <select id="refresh-interval">
	        <option value="2000">2 seconds</option>
	        <option value="5000" selected>5 seconds</option>
	        <option value="10000">10 seconds</option>
	        <option value="30000">30 seconds</option>
	      </select>
	    </label>
	  </div>

	  <table>
	    <caption>Active Sessions</caption>
	    <thead>
	      <tr>
	        <th>Session ID</th>
	        <th>User ID</th>
	        <th>Status</th>
	        <th>Format</th>
	        <th>Sample Rate</th>
	        <th>Start Time</th>
	        <th>Last Activity</th>
	      </tr>
	    </thead>
	    <tbody id="sessions-table"></tbody>
	  </table>

	  <div class="timestamp">Last updated: <span id="last-update">Never</span></div>

	  <script>
	    let refreshInterval;
	    const autoRefreshCheckbox = document.getElementById('auto-refresh');
	    const refreshIntervalSelect = document.getElementById('refresh-interval');

	    function formatDateTime(dateStr) {
	      const date = new Date(dateStr);
	      return date.toLocaleString();
	    }

	    function formatDuration(startTimeStr, endTimeStr) {
	      const start = new Date(startTimeStr).getTime();
	      const end = endTimeStr ? new Date(endTimeStr).getTime() : Date.now();
	      const diffSec = Math.floor((end - start) / 1000);

	      if (diffSec < 60) return diffSec + ' sec';
	      if (diffSec < 3600) return Math.floor(diffSec / 60) + ' min ' + (diffSec % 60) + ' sec';
	      return Math.floor(diffSec / 3600) + ' hr ' + Math.floor((diffSec % 3600) / 60) + ' min';
	    }

	    async function fetchData() {
	      try {
	        const response = await fetch('./monitor/data');
	        if (!response.ok) {
	          console.error('Failed to fetch data:', response.status);
	          return;
	        }

	        const data = await response.json();
	        document.getElementById('sessions-count').textContent = data.active_sessions;
	        document.getElementById('users-count').textContent = data.active_users;
	        document.getElementById('last-update').textContent = new Date().toLocaleString();

	        const tbody = document.getElementById('sessions-table');
	        tbody.innerHTML = '';

	        const sessions = data.sessions;
	        const sessionIds = Object.keys(sessions);

	        if (sessionIds.length === 0) {
	          const row = document.createElement('tr');
	          const cell = document.createElement('td');
	          cell.colSpan = 7;
	          cell.className = 'empty-row';
	          cell.textContent = 'No active sessions';
	          row.appendChild(cell);
	          tbody.appendChild(row);
	        } else {
	          sessionIds.forEach(sessionId => {
	            const session = sessions[sessionId];
	            const row = document.createElement('tr');

	            const cells = [
	              sessionId,
	              session.userID,
	              session.started ? 'Active' : 'Connecting',
	              session.format || 'N/A',
	              session.sampleRate || 'N/A',
	              formatDateTime(session.startTime),
	              formatDateTime(session.lastActivityTime)
	            ];

	            cells.forEach(cellText => {
	              const cell = document.createElement('td');
	              cell.textContent = cellText;
	              row.appendChild(cell);
	            });

	            tbody.appendChild(row);
	          });
	        }
	      } catch (err) {
	        console.error('Error fetching data:', err);
	      }
	    }

	    function updateRefreshInterval() {
	      clearInterval(refreshInterval);
	      if (autoRefreshCheckbox.checked) {
	        const interval = parseInt(refreshIntervalSelect.value);
	        refreshInterval = setInterval(fetchData, interval);
	      }
	    }

	    autoRefreshCheckbox.addEventListener('change', updateRefreshInterval);
	    refreshIntervalSelect.addEventListener('change', updateRefreshInterval);

	    // Initial fetch
	    fetchData();
	    updateRefreshInterval();
	  </script>
	</body>
	</html>
	`
	return c.Type("html").SendString(html)
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

	// Регистрация маршрутов
	router.Get("/", wsMiddleware, wsHandler)
	router.Get("/monitor", ctrl.HandleMonitorPage)
	router.Get("/monitor/data", ctrl.HandleMonitorData)
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
