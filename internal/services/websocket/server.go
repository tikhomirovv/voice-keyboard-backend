package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// Server представляет WebSocket-сервер для аудиостриминга
type Server struct {
	config *pkg.Config
	logger logger.Logger

	// websocket
	upgrader   websocket.Upgrader
	httpServer *http.Server
	vl         *validator.Validate

	// sessions
	sessions     map[string]*WSSession
	userSessions map[uint64]map[string]bool // UserID -> map[SessionID]bool
	sessionMutex sync.RWMutex

	// services
	authService interfaces.AuthServiceInterface
	processor   interfaces.WebSocketProcessorInterface
}

// NewServer создает новый WebSocket-сервер
func NewServer(c *pkg.Container) *Server {
	// Создаем сервисы для обработки WebSocket сообщений
	processor := NewRealtimeWebSocketService(
		c.Logger,
		c.RealtimeTranscriberService,
		c.OpenAITextGenerationService,
	)

	// processor := NewFileWebSocketService(
	// 	c.Logger,
	// 	c.AudioService,
	// 	c.OpenAITextGenerationService,
	// )

	return &Server{
		config: c.Config,
		logger: c.Logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  WebSocketReadBufferSize,
			WriteBufferSize: WebSocketWriteBufferSize,
			CheckOrigin: func(r *http.Request) bool {
				// Здесь может быть проверка origin
				return true
			},
		},
		vl:           c.Validator,
		sessions:     make(map[string]*WSSession),
		userSessions: make(map[uint64]map[string]bool),
		authService:  c.AuthService,
		processor:    processor,
	}
}

// Start запускает WebSocket-сервер
func (s *Server) Start() error {
	if !s.config.WebSocket.Enabled {
		s.logger.Info("WebSocket server is disabled in configuration")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws-monitor/data", s.handleMonitorData)
	mux.HandleFunc("/ws-monitor", s.handleMonitorPage)
	mux.HandleFunc(s.config.WebSocket.Path, s.handleWebSocket)

	s.httpServer = &http.Server{
		Addr:    s.config.GetWebSocketPort(),
		Handler: mux,
	}

	s.logger.Info(fmt.Sprintf("Starting WebSocket server on %s", s.config.GetWebSocketPort()))

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebSocket server error: " + err.Error())
		}
	}()

	return nil
}

// Stop останавливает WebSocket-сервер
func (s *Server) Stop() error {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Закрываем все активные соединения корректно
		s.sessionMutex.Lock()
		s.logger.Info("Gracefully closing all active WebSocket connections")
		for _, session := range s.sessions {
			if session.Conn != nil {
				s.gracefulCloseConn(session.Conn)
			}
		}
		s.sessions = make(map[string]*WSSession)
		s.userSessions = make(map[uint64]map[string]bool)
		s.sessionMutex.Unlock()

		// Останавливаем HTTP-сервер
		s.logger.Info("Stopping WebSocket server")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// handleWebSocket обрабатывает новое WebSocket-соединение
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Проверка авторизации по заголовку Authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization required", http.StatusUnauthorized)
		return
	}

	// Обычно заголовок имеет формат "Bearer <token>"
	tokenString := authHeader[7:] // Удаляем "Bearer "
	userID, err := s.validateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	wsConnectAudioDTO := &dto.WsConnectAudioDTO{}
	wsConnectAudioDTO.SessionID = r.URL.Query().Get("sessionId")
	wsConnectAudioDTO.Format = r.URL.Query().Get("format")
	sampleRateStr := r.URL.Query().Get("sampleRate")
	if sampleRateStr != "" {
		sampleRateUint64, err := strconv.ParseUint(sampleRateStr, 10, 32)
		if err == nil {
			wsConnectAudioDTO.SampleRate = uint32(sampleRateUint64)
		}
	}

	if err := s.vl.Struct(wsConnectAudioDTO); err != nil {
		errs := err.(validator.ValidationErrors)
		s.logger.Error("Ws connect audio validation failed", "error", errs)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Проверка количества соединений для пользователя
	if !s.checkUserConnectionLimit(userID) {
		http.Error(w, "Too many connections for user", http.StatusTooManyRequests)
		return
	}

	// Проверяем уникальность sessionID до апгрейда соединения
	// Это важно делать ДО апгрейда, чтобы:
	// 1. Экономить ресурсы на создание WebSocket-соединения
	// 2. Иметь возможность вернуть клиенту HTTP-статус 409 Conflict
	s.sessionMutex.RLock()
	if _, exists := s.sessions[wsConnectAudioDTO.SessionID]; exists {
		s.sessionMutex.RUnlock()
		http.Error(w, "Session with this ID already exists", http.StatusConflict)
		return
	}
	s.sessionMutex.RUnlock()

	// Апгрейд HTTP-соединения до WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to upgrade connection: %v", err))
		return
	}

	// Инициализируем сессию
	session, err := s.initSession(userID, conn, wsConnectAudioDTO)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Session initialization failed: %v", err))
		conn.Close() // Закрываем соединение при ошибке
		// Отправка ошибки через WebSocket не получится, так как сессия не инициализирована
		// Поэтому просто логируем ошибку и закрываем соединение
		return
	}

	// Запуск обработки сообщений
	go s.handleSession(session)
}

// checkUserConnectionLimit проверяет, не превышен ли лимит соединений для пользователя
func (s *Server) checkUserConnectionLimit(userID uint64) bool {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	sessions, exists := s.userSessions[userID]
	if !exists {
		return true
	}

	return len(sessions) < s.config.WebSocket.MaxConnectionsPerUser
}

// initSession инициализирует новую WebSocket сессию
// Возвращает созданную сессию и ошибку, если что-то пошло не так
func (s *Server) initSession(userID uint64, conn *websocket.Conn, wsConnectAudioDTO *dto.WsConnectAudioDTO) (*WSSession, error) {
	// Создание новой сессии
	session := &WSSession{
		ID:                wsConnectAudioDTO.SessionID,
		UserID:            userID,
		Conn:              conn,
		StartTime:         time.Now(),
		LastActivityTime:  time.Now(),
		Started:           false, // Инициализируем флаг сессии как не начатую
		subscriptionOnce:  sync.Once{},
		subscriptionValid: false, // По умолчанию считаем подписку недействительной
		Mutex:             sync.Mutex{},
		AudioOptions: WSSessionAudioOptions{
			SampleFormat: wsConnectAudioDTO.Format,
			SampleRate:   wsConnectAudioDTO.SampleRate,
		},
	}

	// Регистрируем сессию в обработчике под блокировкой
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	// Регистрируем сессию в картах сервера
	s.sessions[session.ID] = session

	// Добавляем сессию в список сессий пользователя
	if s.userSessions[session.UserID] == nil {
		s.userSessions[session.UserID] = make(map[string]bool)
	}
	s.userSessions[session.UserID][session.ID] = true

	// Асинхронно проверяем подписку пользователя сразу при создании сессии
	s.startCheckSubscription(session)

	s.logger.Info(fmt.Sprintf("Session successfully initialized with ID: %s for user: %d", session.ID, userID))

	return session, nil
}

// handleSession обрабатывает все сообщения WebSocket в рамках одной сессии
func (s *Server) handleSession(session *WSSession) {
	defer s.closeSession(session)

	// Устанавливаем начальный таймаут для чтения
	session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.ConnectionTimeout)))

	// Главный цикл обработки сообщений
	for {
		// Чтение сообщения
		_, message, err := session.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error(fmt.Sprintf("Error reading message: %v", err))
			}
			break
		}

		// Обновляем время последней активности и таймауты
		session.LastActivityTime = time.Now()
		session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))
		session.Conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))

		// Разбор сообщения
		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			s.logger.Error(fmt.Sprintf("Error parsing message: %v", err))
			s.sendError(session, ErrorCodeParseError, ErrorMessageInvalidMessageFormat)
			continue
		}

		// Проверяем соответствие ID сессии
		if wsMessage.SessionID != session.ID {
			s.logger.Error(fmt.Sprintf("Session ID mismatch: %s != %s", wsMessage.SessionID, session.ID))
			s.sendError(session, ErrorCodeSessionError, ErrorMessageSessionIDMismatch)
			continue
		}

		// Обработка сообщения в зависимости от его типа
		switch wsMessage.Type {
		case MessageTypeAudio:
			s.handleAudioMessage(session, wsMessage)
		case MessageTypeStop:
			s.handleStopMessage(session, wsMessage)
			// Для стоп-сообщения завершаем обработку после его выполнения
			return
		default:
			s.logger.Warn(fmt.Sprintf("Unknown message type: %s", wsMessage.Type))
			s.sendError(session, ErrorCodeUnknownType, ErrorMessageUnknownMessageType)
		}
	}
}

// handleAudioMessage обрабатывает сообщение с аудиоданными
func (s *Server) handleAudioMessage(session *WSSession, message WebSocketMessage) {
	session.Mutex.Lock()

	// Проверяем, начата ли сессия
	if !session.Started {
		// Если сессия не начата, считаем это началом сессии
		session.Started = true
		session.StartTime = time.Now() // Сбрасываем время начала

		// Инициализируем сессию
		err := s.processor.StartSession(session.ID, &interfaces.ProcessorSessionOptions{
			UserID:     session.UserID,
			Format:     session.AudioOptions.SampleFormat,
			SampleRate: session.AudioOptions.SampleRate,
			Callback: func(text string) {
				s.sendPartialMessage(session, text)
			},
		})
		if err != nil {
			session.Mutex.Unlock()
			s.logger.Error(fmt.Sprintf("Failed to start session: %v", err))
			s.sendError(session, ErrorCodeSessionError, ErrorMessageFailedToStartSession)
			return
		}

		s.logger.Info(fmt.Sprintf("Started realtime transcription session: %s for user: %d", session.ID, session.UserID))
	}

	session.Mutex.Unlock()

	// Обрабатываем аудиосообщение
	if err := s.processor.HandleAudioMessage(session.ID, message.Data); err != nil {
		s.logger.Error(fmt.Sprintf("Error handling audio message: %v", err))
		s.sendError(session, ErrorCodeProcessingError, ErrorMessageFailedToProcessAudio)
		return
	}
}

// handleStopMessage обрабатывает сообщение об окончании записи
func (s *Server) handleStopMessage(session *WSSession, _ WebSocketMessage) {
	// Блокируем сессию для проверки
	session.Mutex.Lock()

	// Проверяем, была ли начата сессия
	if !session.Started {
		session.Mutex.Unlock()
		s.logger.Warn(fmt.Sprintf("Attempt to stop not started session: %s for user: %d", session.ID, session.UserID))
		s.sendError(session, ErrorCodeSessionError, ErrorMessageSessionNotStarted)
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	// Освобождаем мьютекс перед блокирующей операцией проверки подписки
	session.Mutex.Unlock()

	// Проверяем подписку пользователя (может заблокироваться на 5 секунд)
	if !s.waitForSubscriptionStatus(session) {
		// Подписка недействительна или произошел таймаут, закрываем соединение
		go s.closeSession(session)
		return
	}

	// Обрабатываем стоп-сообщение
	text, err := s.processor.HandleStopMessage(session.ID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error handling stop message: %v", err))
		s.sendError(session, ErrorCodeProcessingError, ErrorMessageFailedToStopMessage)
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	if err := s.sendCompletedMessage(session, text); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending completed message: %v", err))
	}

	// Закрываем соединение после отправки результата
	s.logger.Info(fmt.Sprintf("Closing connection after processing session: %s for user: %d", session.ID, session.UserID))
	go s.closeSession(session)
}

// Helpers for sending messages ------------------------------------------------

// isSessionActive проверяет, активна ли сессия (существует ли в списке сессий)
func (s *Server) isSessionActive(sessionID string) bool {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()
	_, exists := s.sessions[sessionID]
	return exists
}

// sendMessage отправляет сообщение клиенту
func (s *Server) sendMessage(session *WSSession, message WebSocketMessage) error {
	// Проверяем, что сессия еще активна
	if !s.isSessionActive(session.ID) {
		return fmt.Errorf("session %s is already closed", session.ID)
	}

	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	return session.Conn.WriteMessage(websocket.TextMessage, data)
}

// sendError отправляет сообщение об ошибке клиенту
func (s *Server) sendError(session *WSSession, code string, message string) {
	errorDataJSON, _ := json.Marshal(ErrorData{
		Code:    code,
		Message: message,
	})
	response := WebSocketMessage{
		Type:      MessageTypeError,
		SessionID: session.ID,
		Data:      errorDataJSON,
	}
	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending error message: %v", err))
	}
}

func (s *Server) sendPartialMessage(session *WSSession, text string) {
	partialDataJSON, _ := json.Marshal(PartialData{
		Text: text,
	})
	response := WebSocketMessage{
		Type:      MessageTypePartial,
		SessionID: session.ID,
		Data:      partialDataJSON,
	}

	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending partial result: %v", err))
	}

	s.logger.Info(fmt.Sprintf("Partial result sent for session: %s, text: %s", session.ID, text))
}

func (s *Server) sendCompletedMessage(session *WSSession, text string) error {
	completedDataJSON, _ := json.Marshal(CompletedData{
		Text: text,
	})
	response := WebSocketMessage{
		Type:      MessageTypeCompleted,
		SessionID: session.ID,
		Data:      completedDataJSON,
	}
	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error("Error sending result: " + err.Error())
		return fmt.Errorf(ErrorMessageFailedToSendResult+": %w", err)
	}

	return nil
}

// CLOSE SESSION

// closeSession закрывает сессию и удаляет её из списка активных сессий
func (s *Server) closeSession(session *WSSession) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	// Проверяем, не была ли сессия уже закрыта
	_, exists := s.sessions[session.ID]
	if !exists {
		// Сессия уже была закрыта, ничего не делаем
		return
	}

	// Закрываем сессию через соответствующий сервис
	if err := s.processor.CloseSession(session.ID); err != nil {
		s.logger.Warn(fmt.Sprintf("Error closing session: %v", err))
	}

	// Корректно закрываем WebSocket соединение
	if session.Conn != nil {
		s.gracefulCloseConn(session.Conn)
	}

	// Удаляем сессию из списка
	delete(s.sessions, session.ID)

	// Удаляем сессию из списка сессий пользователя
	if sessions, exists := s.userSessions[session.UserID]; exists {
		delete(sessions, session.ID)
		if len(sessions) == 0 {
			delete(s.userSessions, session.UserID)
		}
	}

	s.logger.Info(fmt.Sprintf("Session closed: %s for user: %d", session.ID, session.UserID))
}

// gracefulCloseConn корректно закрывает WebSocket соединение в соответствии с протоколом
// Отправляет Close фрейм и ждет подтверждения от клиента с таймаутом
func (s *Server) gracefulCloseConn(conn *websocket.Conn) {
	// Устанавливаем дедлайн для закрытия соединения
	deadline := time.Now().Add(GracefulCloseTimeout)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to set write deadline for close: %v", err))
	}

	// Отправляем фрейм Close с кодом и сообщением
	// 1000 - нормальное закрытие, "closing" - причина закрытия
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, CloseMessageText)
	if err := conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		s.logger.Warn(fmt.Sprintf("Error sending close message: %v", err))
		// Если не удалось отправить close фрейм, просто закрываем соединение
		conn.Close()
		return
	}

	// Устанавливаем таймаут для чтения ответа от клиента
	if err := conn.SetReadDeadline(deadline); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to set read deadline for close response: %v", err))
	}

	// Ожидаем подтверждения закрытия от клиента (или таймаута)
	// Это позволит корректно завершить WebSocket handshake
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

// USER AUTH AND SUBSCRIPTION ----------------------------------------------------------------

// validateToken проверяет JWT-токен и возвращает ID пользователя в виде строки
func (s *Server) validateToken(tokenString string) (uint64, error) {
	s.logger.Debug(fmt.Sprintf("Validating token: %s", tokenString))
	// Используем AuthService для проверки токена
	userID, err := s.authService.ValidateToken(tokenString)
	if err != nil {
		return 0, fmt.Errorf("token validation error: %w", err)
	}

	// Преобразуем ID пользователя в строку
	return userID, nil
}

// getUserSubscription проверяет наличие активной подписки у пользователя
// Возвращает SubscriptionDTO с результатом проверки подписки
// В текущей реализации это заглушка, которую нужно заменить на реальную проверку
func (s *Server) getUserSubscription(userID uint64) (*dto.SubscriptionDTO, error) {
	s.logger.Debug(fmt.Sprintf("Checking subscription for user: %d", userID))
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
// и сохраняет результат в session.subscriptionValid
func (s *Server) startCheckSubscription(session *WSSession) {
	// Запускаем проверку в горутине, чтобы не блокировать основной поток
	go func() {
		session.subscriptionOnce.Do(func() {
			s.logger.Debug(fmt.Sprintf("Checking subscription for user: %d", session.UserID))
			subscription, err := s.getUserSubscription(session.UserID)

			if err != nil {
				s.logger.Error(fmt.Sprintf("Failed to check subscription: %v", err))
				// При ошибке проверки считаем, что подписки нет
				session.Mutex.Lock()
				session.subscriptionValid = false
				session.Mutex.Unlock()

				// Отправляем сообщение об ошибке
				s.sendError(session, ErrorCodeSubscriptionError, ErrorMessageFailedToCheckSubscription)

				// Закрываем соединение только при ошибке проверки
				go s.closeSession(session)
				return
			}

			// Сохраняем результат проверки под защитой мьютекса
			session.Mutex.Lock()
			session.subscriptionValid = subscription.IsValid
			session.Mutex.Unlock()

			if !subscription.IsValid {
				s.logger.Warn(fmt.Sprintf("User has no valid subscription: %d", session.UserID))
				s.sendError(session, ErrorCodeSubscriptionError, ErrorMessageUserNoValidSubscription)
				go s.closeSession(session)

			} else {
				s.logger.Info(fmt.Sprintf("Valid subscription confirmed for user: %d", session.UserID))
			}
		})
	}()
}

// waitForSubscriptionStatus ожидает результат проверки подписки с таймаутом
// Возвращает true, если у пользователя есть действительная подписка, false в противном случае
func (s *Server) waitForSubscriptionStatus(session *WSSession) bool {
	s.logger.Info(fmt.Sprintf("Checking subscription result for session: %s", session.ID))

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
			s.logger.Info(fmt.Sprintf("Subscription valid for session: %s", session.ID))
			return true
		} else {
			s.logger.Warn(fmt.Sprintf("Subscription invalid for session: %s for user: %d", session.ID, session.UserID))
			s.sendError(session, ErrorCodeSubscriptionError, ErrorMessageValidSubscriptionRequired)
			return false
		}

	case <-time.After(SubscriptionCheckTimeout):
		// Таймаут ожидания проверки подписки
		s.logger.Error(fmt.Sprintf("Subscription check timeout for session: %s", session.ID))
		s.sendError(session, ErrorCodeSubscriptionError, ErrorMessageSubscriptionCheckTimeout)
		return false
	}
}
