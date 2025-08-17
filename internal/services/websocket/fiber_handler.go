package websocket

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/websocket/v2"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// FiberWSSession представляет активную сессию WebSocket для Fiber
type FiberWSSession struct {
	ID                string
	UserID            uint64
	Conn              *websocket.Conn
	StartTime         time.Time
	LastActivityTime  time.Time
	Started           bool
	subscriptionOnce  sync.Once
	subscriptionValid bool
	Mutex             sync.Mutex
	AudioOptions      WSSessionAudioOptions
}

// FiberHandler представляет WebSocket обработчик для Fiber
type FiberHandler struct {
	config *pkg.Config
	logger logger.Logger

	// sessions
	sessions     map[string]*FiberWSSession
	userSessions map[uint64]map[string]bool // UserID -> map[SessionID]bool
	sessionMutex sync.RWMutex

	// services
	authService interfaces.AuthServiceInterface
	processor   interfaces.WebSocketProcessorInterface
	validator   *validator.Validate
}

// GetSessionsCount возвращает количество активных сессий
func (h *FiberHandler) GetSessionsCount() int {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()
	return len(h.sessions)
}

// GetUsersCount возвращает количество пользователей с активными сессиями
func (h *FiberHandler) GetUsersCount() int {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()
	return len(h.userSessions)
}

// GetSessionsInfo возвращает информацию о сессиях для мониторинга
func (h *FiberHandler) GetSessionsInfo() map[string]interface{} {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()

	info := make(map[string]interface{})
	for sessionID, session := range h.sessions {
		info[sessionID] = map[string]interface{}{
			"userID":           session.UserID,
			"startTime":        session.StartTime,
			"lastActivityTime": session.LastActivityTime,
			"started":          session.Started,
			"format":           session.AudioOptions.SampleFormat,
			"sampleRate":       session.AudioOptions.SampleRate,
		}
	}

	return info
}

// NewFiberHandler создает новый WebSocket обработчик для Fiber
func NewFiberHandler(c *pkg.Container, processor interfaces.WebSocketProcessorInterface) *FiberHandler {
	// Используем предоставленный процессор для обработки WebSocket сообщений

	return &FiberHandler{
		config:       c.Config,
		logger:       c.Logger,
		sessions:     make(map[string]*FiberWSSession),
		userSessions: make(map[uint64]map[string]bool),
		authService:  c.AuthService,
		processor:    processor,
		validator:    c.Validator,
	}
}

// Методы для обработки WebSocket находятся в WebSocketController

// HandleWebSocket обрабатывает WebSocket соединение
func (h *FiberHandler) HandleWebSocket(c *websocket.Conn) {
	// Получаем userID из контекста
	userID, ok := c.Locals("userID").(uint64)
	if !ok {
		h.logger.Error("UserID not found in context")
		c.Close()
		return
	}

	// Получаем параметры из query string
	sessionID := c.Query("sessionId")
	format := c.Query("format")
	sampleRateStr := c.Query("sampleRate")

	// Валидация параметров
	wsConnectAudioDTO := &dto.WsConnectAudioDTO{
		SessionID: sessionID,
		Format:    format,
	}

	if sampleRateStr != "" {
		sampleRateUint64, err := strconv.ParseUint(sampleRateStr, 10, 32)
		if err == nil {
			wsConnectAudioDTO.SampleRate = uint32(sampleRateUint64)
		}
	}

	if err := h.validator.Struct(wsConnectAudioDTO); err != nil {
		h.logger.Error("Ws connect audio validation failed", "error", err)
		h.sendError(c, ErrorCodeValidationError, "Invalid request parameters")
		c.Close()
		return
	}

	// Проверка количества соединений для пользователя
	if !h.checkUserConnectionLimit(userID) {
		h.sendError(c, ErrorCodeLimitExceeded, "Too many connections for user")
		c.Close()
		return
	}

	// Проверяем уникальность sessionID
	h.sessionMutex.RLock()
	if _, exists := h.sessions[wsConnectAudioDTO.SessionID]; exists {
		h.sessionMutex.RUnlock()
		h.sendError(c, ErrorCodeSessionError, "Session with this ID already exists")
		c.Close()
		return
	}
	h.sessionMutex.RUnlock()

	// Инициализируем сессию
	session, err := h.initSession(userID, c, wsConnectAudioDTO)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Session initialization failed: %v", err))
		h.sendError(c, ErrorCodeSessionError, "Failed to initialize session")
		c.Close()
		return
	}

	// Запуск обработки сообщений
	h.handleSession(session)
}

// initSession инициализирует новую WebSocket сессию
func (h *FiberHandler) initSession(userID uint64, conn *websocket.Conn, wsConnectAudioDTO *dto.WsConnectAudioDTO) (*FiberWSSession, error) {
	// Создание новой сессии
	session := &FiberWSSession{
		ID:                wsConnectAudioDTO.SessionID,
		UserID:            userID,
		Conn:              conn,
		StartTime:         time.Now(),
		LastActivityTime:  time.Now(),
		Started:           false,
		subscriptionOnce:  sync.Once{},
		subscriptionValid: false,
		Mutex:             sync.Mutex{},
		AudioOptions: WSSessionAudioOptions{
			SampleFormat: wsConnectAudioDTO.Format,
			SampleRate:   wsConnectAudioDTO.SampleRate,
		},
	}

	// Регистрируем сессию в обработчике под блокировкой
	h.sessionMutex.Lock()
	defer h.sessionMutex.Unlock()

	// Регистрируем сессию в картах сервера
	h.sessions[session.ID] = session

	// Добавляем сессию в список сессий пользователя
	if h.userSessions[session.UserID] == nil {
		h.userSessions[session.UserID] = make(map[string]bool)
	}
	h.userSessions[session.UserID][session.ID] = true

	// Асинхронно проверяем подписку пользователя сразу при создании сессии
	h.startCheckSubscription(session)

	h.logger.Info(fmt.Sprintf("Session successfully initialized with ID: %s for user: %d", session.ID, session.UserID))

	return session, nil
}

// handleSession обрабатывает все сообщения WebSocket в рамках одной сессии
func (h *FiberHandler) handleSession(session *FiberWSSession) {
	defer h.closeSession(session)

	// Устанавливаем начальный таймаут для чтения
	session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(h.config.WebSocket.ConnectionTimeout)))

	// Главный цикл обработки сообщений
	for {
		// Чтение сообщения
		_, message, err := session.Conn.ReadMessage()
		if err != nil {
			h.logger.Error(fmt.Sprintf("Error reading message: %v", err))
			break
		}

		// Обновляем время последней активности и таймауты
		session.LastActivityTime = time.Now()
		session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(h.config.WebSocket.IdleTimeout)))
		session.Conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(h.config.WebSocket.IdleTimeout)))

		// Разбор сообщения
		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			h.logger.Error(fmt.Sprintf("Error parsing message: %v", err))
			h.sendErrorToSession(session, ErrorCodeParseError, ErrorMessageInvalidMessageFormat)
			continue
		}

		// Проверяем соответствие ID сессии
		if wsMessage.SessionID != session.ID {
			h.logger.Error(fmt.Sprintf("Session ID mismatch: %s != %s", wsMessage.SessionID, session.ID))
			h.sendErrorToSession(session, ErrorCodeSessionError, ErrorMessageSessionIDMismatch)
			continue
		}

		// Обработка сообщения в зависимости от его типа
		switch wsMessage.Type {
		case MessageTypeAudio:
			h.handleAudioMessage(session, wsMessage)
		case MessageTypeStop:
			h.handleStopMessage(session, wsMessage)
			// Для стоп-сообщения завершаем обработку после его выполнения
			return
		default:
			h.logger.Warn(fmt.Sprintf("Unknown message type: %s", wsMessage.Type))
			h.sendErrorToSession(session, ErrorCodeUnknownType, ErrorMessageUnknownMessageType)
		}
	}
}

// checkUserConnectionLimit проверяет, не превышен ли лимит соединений для пользователя
func (h *FiberHandler) checkUserConnectionLimit(userID uint64) bool {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()

	sessions, exists := h.userSessions[userID]
	if !exists {
		return true
	}

	return len(sessions) < h.config.WebSocket.MaxConnectionsPerUser
}

// validateToken проверяет JWT-токен и возвращает ID пользователя
func (h *FiberHandler) validateToken(tokenString string) (uint64, error) {
	h.logger.Debug(fmt.Sprintf("Validating token: %s", tokenString))
	// Используем AuthService для проверки токена
	userID, err := h.authService.ValidateToken(tokenString)
	if err != nil {
		return 0, fmt.Errorf("token validation error: %w", err)
	}

	return userID, nil
}

// sendError отправляет ошибку через WebSocket
func (h *FiberHandler) sendError(conn *websocket.Conn, code, message string) {
	errorData := ErrorData{
		Code:    code,
		Message: message,
	}

	response := WebSocketMessage{
		Type: MessageTypeError,
		Data: func() json.RawMessage {
			data, _ := json.Marshal(errorData)
			return data
		}(),
	}

	responseData, _ := json.Marshal(response)
	conn.WriteMessage(websocket.TextMessage, responseData)
}

// sendError для сессии
func (h *FiberHandler) sendErrorToSession(session *FiberWSSession, code, message string) {
	h.sendError(session.Conn, code, message)
}

// handleAudioMessage обрабатывает сообщение с аудиоданными
func (h *FiberHandler) handleAudioMessage(session *FiberWSSession, message WebSocketMessage) {
	session.Mutex.Lock()

	// Проверяем, начата ли сессия
	if !session.Started {
		// Если сессия не начата, считаем это началом сессии
		session.Started = true
		session.StartTime = time.Now() // Сбрасываем время начала

		// Инициализируем сессию
		err := h.processor.StartSession(session.ID, &interfaces.ProcessorSessionOptions{
			UserID:     session.UserID,
			Format:     session.AudioOptions.SampleFormat,
			SampleRate: session.AudioOptions.SampleRate,
			Callback: func(text string) {
				h.sendPartialMessage(session, text)
			},
		})
		if err != nil {
			session.Mutex.Unlock()
			h.logger.Error(fmt.Sprintf("Failed to start session: %v", err))
			h.sendErrorToSession(session, ErrorCodeSessionError, ErrorMessageFailedToStartSession)
			return
		}

		h.logger.Info(fmt.Sprintf("Started realtime transcription session: %s for user: %d", session.ID, session.UserID))
	}

	session.Mutex.Unlock()

	// Обрабатываем аудиосообщение
	if err := h.processor.HandleAudioMessage(session.ID, message.Data); err != nil {
		h.logger.Error(fmt.Sprintf("Error handling audio message: %v", err))
		h.sendErrorToSession(session, ErrorCodeProcessingError, ErrorMessageFailedToProcessAudio)
		return
	}
}

// handleStopMessage обрабатывает сообщение об окончании записи
func (h *FiberHandler) handleStopMessage(session *FiberWSSession, _ WebSocketMessage) {
	// Блокируем сессию для проверки
	session.Mutex.Lock()

	// Проверяем, была ли начата сессия
	if !session.Started {
		session.Mutex.Unlock()
		h.logger.Warn(fmt.Sprintf("Attempt to stop not started session: %s for user: %d", session.ID, session.UserID))
		h.sendErrorToSession(session, ErrorCodeSessionError, ErrorMessageSessionNotStarted)
		// Закрываем соединение после ошибки
		go h.closeSession(session)
		return
	}

	// Освобождаем мьютекс перед блокирующей операцией проверки подписки
	session.Mutex.Unlock()

	// Проверяем подписку пользователя (может заблокироваться на 5 секунд)
	if !h.waitForSubscriptionStatus(session) {
		// Подписка недействительна или произошел таймаут, закрываем соединение
		go h.closeSession(session)
		return
	}

	// Обрабатываем стоп-сообщение
	text, err := h.processor.HandleStopMessage(session.ID)
	if err != nil {
		h.logger.Error(fmt.Sprintf("Error handling stop message: %v", err))
		h.sendErrorToSession(session, ErrorCodeProcessingError, ErrorMessageFailedToStopMessage)
		// Закрываем соединение после ошибки
		go h.closeSession(session)
		return
	}

	if err := h.sendCompletedMessage(session, text); err != nil {
		h.logger.Error(fmt.Sprintf("Error sending completed message: %v", err))
	}

	// Закрываем соединение после отправки результата
	h.logger.Info(fmt.Sprintf("Closing connection after processing session: %s for user: %d", session.ID, session.UserID))
	go h.closeSession(session)
}

// sendPartialMessage отправляет частичное сообщение
func (h *FiberHandler) sendPartialMessage(session *FiberWSSession, text string) {
	// Проверяем, что сессия еще активна
	if !h.isSessionActive(session.ID) {
		h.logger.Warn(fmt.Sprintf("Attempt to send partial message to closed session: %s", session.ID))
		return
	}

	partialData := PartialData{
		Text: text,
	}

	response := WebSocketMessage{
		Type: MessageTypePartial,
		Data: func() json.RawMessage {
			data, _ := json.Marshal(partialData)
			return data
		}(),
	}

	responseData, _ := json.Marshal(response)
	if err := session.Conn.WriteMessage(websocket.TextMessage, responseData); err != nil {
		h.logger.Error(fmt.Sprintf("Error sending partial result: %v", err))
	} else {
		h.logger.Info(fmt.Sprintf("Partial result sent for session: %s, text: %s", session.ID, text))
	}
}

// sendCompletedMessage отправляет завершенное сообщение
func (h *FiberHandler) sendCompletedMessage(session *FiberWSSession, text string) error {
	completedData := CompletedData{
		Text: text,
	}

	response := WebSocketMessage{
		Type: MessageTypeCompleted,
		Data: func() json.RawMessage {
			data, _ := json.Marshal(completedData)
			return data
		}(),
	}

	responseData, _ := json.Marshal(response)
	return session.Conn.WriteMessage(websocket.TextMessage, responseData)
}

// isSessionActive проверяет, активна ли сессия (существует ли в списке сессий)
func (h *FiberHandler) isSessionActive(sessionID string) bool {
	h.sessionMutex.RLock()
	defer h.sessionMutex.RUnlock()
	_, exists := h.sessions[sessionID]
	return exists
}

// closeSession закрывает сессию и удаляет её из списка активных сессий
func (h *FiberHandler) closeSession(session *FiberWSSession) {
	h.sessionMutex.Lock()
	defer h.sessionMutex.Unlock()

	// Проверяем, не была ли сессия уже закрыта
	_, exists := h.sessions[session.ID]
	if !exists {
		// Сессия уже была закрыта, ничего не делаем
		return
	}

	// Закрываем сессию через соответствующий сервис
	if err := h.processor.CloseSession(session.ID); err != nil {
		h.logger.Warn(fmt.Sprintf("Error closing session: %v", err))
	}

	// Корректно закрываем WebSocket соединение
	if session.Conn != nil {
		h.gracefulCloseConn(session.Conn)
	}

	// Удаляем сессию из списка
	delete(h.sessions, session.ID)

	// Удаляем сессию из списка сессий пользователя
	if sessions, exists := h.userSessions[session.UserID]; exists {
		delete(sessions, session.ID)
		if len(sessions) == 0 {
			delete(h.userSessions, session.UserID)
		}
	}

	h.logger.Info(fmt.Sprintf("Session closed: %s for user: %d", session.ID, session.UserID))
}

// gracefulCloseConn корректно закрывает WebSocket соединение в соответствии с протоколом
func (h *FiberHandler) gracefulCloseConn(conn *websocket.Conn) {
	// Устанавливаем дедлайн для закрытия соединения
	deadline := time.Now().Add(GracefulCloseTimeout)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		h.logger.Warn(fmt.Sprintf("Failed to set write deadline for close: %v", err))
	}

	// Отправляем фрейм Close с кодом и сообщением
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, CloseMessageText)
	if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		h.logger.Warn(fmt.Sprintf("Error sending close message: %v", err))
		// Если не удалось отправить close фрейм, просто закрываем соединение
		conn.Close()
		return
	}

	// Устанавливаем таймаут для чтения ответа от клиента
	if err := conn.SetReadDeadline(deadline); err != nil {
		h.logger.Warn(fmt.Sprintf("Failed to set read deadline for close response: %v", err))
	}

	// Ожидаем подтверждения закрытия от клиента (или таймаута)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// Нормальное закрытие или таймаут
			conn.Close()
			return
		}
		// Если получили сообщение, продолжаем читать до ошибки или таймаута
	}
}

// getUserSubscription проверяет наличие активной подписки у пользователя
func (h *FiberHandler) getUserSubscription(userID uint64) (*dto.SubscriptionDTO, error) {
	h.logger.Debug(fmt.Sprintf("Checking subscription for user: %d", userID))
	// Имитация задержки при проверке подписки
	time.Sleep(1000 * time.Millisecond)

	// FIXME: В реальной реализации здесь должен быть код для:
	// 1. Получения информации о пользователе из базы данных по userID
	// 2. Проверки наличия активной подписки в БД
	// 3. Проверки лимитов использования (например, количество минут в месяц)
	// 4. Возможно, обновления счетчиков использования

	// Заглушка: считаем, что у всех пользователей есть подписка
	return &dto.SubscriptionDTO{
		IsValid: true,
	}, nil
}

// startCheckSubscription запускает асинхронную проверку подписки пользователя
func (h *FiberHandler) startCheckSubscription(session *FiberWSSession) {
	go func() {
		session.subscriptionOnce.Do(func() {
			h.logger.Debug(fmt.Sprintf("Checking subscription for user: %d", session.UserID))
			subscription, err := h.getUserSubscription(session.UserID)

			if err != nil {
				h.logger.Error(fmt.Sprintf("Failed to check subscription: %v", err))
				// При ошибке проверки считаем, что подписки нет
				session.Mutex.Lock()
				session.subscriptionValid = false
				session.Mutex.Unlock()

				// Отправляем сообщение об ошибке
				h.sendErrorToSession(session, ErrorCodeSubscriptionError, ErrorMessageFailedToCheckSubscription)

				// Закрываем соединение только при ошибке проверки
				go h.closeSession(session)
				return
			}

			// Сохраняем результат проверки под защитой мьютекса
			session.Mutex.Lock()
			session.subscriptionValid = subscription.IsValid
			session.Mutex.Unlock()

			if !subscription.IsValid {
				h.logger.Warn(fmt.Sprintf("User has no valid subscription: %d", session.UserID))
				h.sendErrorToSession(session, ErrorCodeSubscriptionError, ErrorMessageUserNoValidSubscription)
				go h.closeSession(session)
			} else {
				h.logger.Info(fmt.Sprintf("Valid subscription confirmed for user: %d", session.UserID))
			}
		})
	}()
}

// waitForSubscriptionStatus ожидает результат проверки подписки с таймаутом
func (h *FiberHandler) waitForSubscriptionStatus(session *FiberWSSession) bool {
	h.logger.Info(fmt.Sprintf("Checking subscription result for session: %s", session.ID))

	// Ждем завершения проверки подписки с таймаутом
	done := make(chan struct{})
	go func() {
		session.subscriptionOnce.Do(func() {})
		close(done)
	}()

	select {
	case <-done:
		// Проверка завершена, читаем результат под защитой мьютекса
		session.Mutex.Lock()
		isValid := session.subscriptionValid
		session.Mutex.Unlock()

		if isValid {
			h.logger.Info(fmt.Sprintf("Subscription valid for session: %s", session.ID))
			return true
		} else {
			h.logger.Warn(fmt.Sprintf("Subscription invalid for session: %s for user: %d", session.ID, session.UserID))
			h.sendErrorToSession(session, ErrorCodeSubscriptionError, ErrorMessageValidSubscriptionRequired)
			return false
		}

	case <-time.After(SubscriptionCheckTimeout):
		// Таймаут ожидания проверки подписки
		h.logger.Error(fmt.Sprintf("Subscription check timeout for session: %s", session.ID))
		h.sendErrorToSession(session, ErrorCodeSubscriptionError, ErrorMessageSubscriptionCheckTimeout)
		return false
	}
}

// Методы мониторинга перенесены в WebSocketController
