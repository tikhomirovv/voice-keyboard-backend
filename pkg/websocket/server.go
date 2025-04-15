package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// MessageType определяет тип сообщения, которое отправляется через WebSocket
type MessageType string

const (
	// Типы сообщений от клиента к серверу
	MessageTypeAudio MessageType = "audio" // Аудиоданные
	MessageTypeStop  MessageType = "stop"  // Конец сессии

	// Типы сообщений от сервера к клиенту
	MessageTypeResult MessageType = "result" // Результат транскрипции
	MessageTypeError  MessageType = "error"  // Ошибка
)

// WebSocketMessage представляет структуру сообщения через WebSocket
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	Data      json.RawMessage `json:"data"`
}

// AudioData содержит аудиоданные в сообщении
type AudioData struct {
	Format  string `json:"format"`  // Формат аудио (например, "wav", "raw")
	Samples []byte `json:"samples"` // Байты аудиосэмплов в base64
}

// ResultData содержит результат распознавания
type ResultData struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
}

// ErrorData содержит информацию об ошибке
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Session представляет активную сессию WebSocket
type Session struct {
	ID                string
	UserID            string
	Conn              *websocket.Conn
	StartTime         time.Time
	LastTime          time.Time
	AudioData         []byte    // Здесь можно хранить накопленные аудиоданные
	Started           bool      // Флаг, указывающий, была ли сессия начата
	subscriptionCheck chan bool // Канал для ожидания завершения проверки подписки
	mutex             sync.Mutex
}

// Server представляет WebSocket-сервер для аудиостриминга
type Server struct {
	config       *pkg.Config
	logger       logger.Logger
	authService  interfaces.AuthServiceInterface
	upgrader     websocket.Upgrader
	sessions     map[string]*Session
	userSessions map[string]map[string]bool // UserID -> map[SessionID]bool
	sessionMutex sync.RWMutex
	httpServer   *http.Server
}

// NewServer создает новый WebSocket-сервер
func NewServer(c *pkg.Container) *Server {
	return &Server{
		config:      c.Config,
		logger:      c.Logger,
		authService: c.AuthService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Здесь может быть проверка origin
				return true
			},
		},
		sessions:     make(map[string]*Session),
		userSessions: make(map[string]map[string]bool),
	}
}

// Start запускает WebSocket-сервер
func (s *Server) Start() error {
	if !s.config.WebSocket.Enabled {
		s.logger.Info("WebSocket server is disabled in configuration")
		return nil
	}

	mux := http.NewServeMux()
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

		// Закрываем все активные соединения
		s.sessionMutex.Lock()
		for _, session := range s.sessions {
			session.Conn.Close()
		}
		s.sessions = make(map[string]*Session)
		s.userSessions = make(map[string]map[string]bool)
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

	// Проверка количества соединений для пользователя
	if !s.checkUserConnectionLimit(userID) {
		http.Error(w, "Too many connections for user", http.StatusTooManyRequests)
		return
	}

	// Апгрейд HTTP-соединения до WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to upgrade connection: %v", err))
		return
	}

	// Создание новой сессии с временным ID, который будет заменен на ID из первого сообщения
	session := &Session{
		ID:                "", // Временный пустой ID, который будет установлен после первого сообщения
		UserID:            userID,
		Conn:              conn,
		StartTime:         time.Now(),
		LastTime:          time.Now(),
		AudioData:         make([]byte, 0),
		Started:           false, // Инициализируем флаг сессии как не начатую
		subscriptionCheck: make(chan bool, 1),
	}

	// Запуск обработки сообщений и регистрация сессии произойдет после получения первого сообщения
	go s.handleSessionInitialization(session)
}

// handleSessionInitialization обрабатывает первое сообщение и инициализирует сессию
func (s *Server) handleSessionInitialization(session *Session) {
	defer s.closeSession(session)

	// Устанавливаем таймаут для чтения
	session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.ConnectionTimeout)))

	// Чтение первого сообщения
	_, message, err := session.Conn.ReadMessage()
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error reading first message: %v", err))
		return
	}

	// Обновляем таймаут
	session.LastTime = time.Now()
	session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))
	session.Conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))

	// Разбор первого сообщения
	var wsMessage WebSocketMessage
	if err := json.Unmarshal(message, &wsMessage); err != nil {
		s.logger.Error(fmt.Sprintf("Error parsing first message: %v", err))
		s.sendError(session, "PARSE_ERROR", "Invalid message format")
		return
	}

	// Проверяем наличие ID сессии в первом сообщении
	if wsMessage.SessionID == "" {
		s.logger.Error("First message must contain sessionId")
		s.sendError(session, "SESSION_ERROR", "First message must contain sessionId")
		return
	}

	// Устанавливаем ID сессии из сообщения
	session.ID = wsMessage.SessionID
	s.logger.Info(fmt.Sprintf("New session initialized with ID: %s for user: %s", session.ID, session.UserID))

	// Регистрируем сессию в хранилище
	s.sessionMutex.Lock()
	// Проверяем, нет ли уже такой сессии
	if _, exists := s.sessions[session.ID]; exists {
		s.sessionMutex.Unlock()
		s.logger.Error(fmt.Sprintf("Session with ID %s already exists", session.ID))
		s.sendError(session, "SESSION_ERROR", "Session with this ID already exists")
		return
	}
	s.sessions[session.ID] = session
	if s.userSessions[session.UserID] == nil {
		s.userSessions[session.UserID] = make(map[string]bool)
	}
	s.userSessions[session.UserID][session.ID] = true
	s.sessionMutex.Unlock()

	// Обрабатываем первое сообщение
	switch wsMessage.Type {
	case MessageTypeAudio:
		s.handleAudioMessage(session, wsMessage)
	case MessageTypeStop:
		s.handleStopMessage(session, wsMessage)
	default:
		s.logger.Warn(fmt.Sprintf("Unknown message type in first message: %s", wsMessage.Type))
		s.sendError(session, "UNKNOWN_TYPE", "Unknown message type")
		return
	}

	// Продолжаем обработку остальных сообщений
	s.handleSessionMessages(session)
}

// handleSessionMessages обрабатывает сообщения в рамках установленной сессии
func (s *Server) handleSessionMessages(session *Session) {
	// Обработка последующих сообщений
	for {
		// Чтение сообщения
		_, message, err := session.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error(fmt.Sprintf("Error reading message: %v", err))
			}
			break
		}

		// Обновляем время последней активности
		session.LastTime = time.Now()
		session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))
		session.Conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))

		// Обработка сообщения
		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			s.logger.Error(fmt.Sprintf("Error parsing message: %v", err))
			s.sendError(session, "PARSE_ERROR", "Invalid message format")
			continue
		}

		// Проверка ID сессии
		if wsMessage.SessionID != session.ID {
			s.logger.Error(fmt.Sprintf("Session ID mismatch: %s != %s", wsMessage.SessionID, session.ID))
			s.sendError(session, "SESSION_ERROR", "Session ID mismatch")
			continue
		}

		// Обработка сообщения в зависимости от его типа
		switch wsMessage.Type {
		case MessageTypeAudio:
			s.handleAudioMessage(session, wsMessage)
		case MessageTypeStop:
			s.handleStopMessage(session, wsMessage)
		default:
			s.logger.Warn(fmt.Sprintf("Unknown message type: %s", wsMessage.Type))
			s.sendError(session, "UNKNOWN_TYPE", "Unknown message type")
		}
	}
}

// validateToken проверяет JWT-токен и возвращает ID пользователя в виде строки
func (s *Server) validateToken(tokenString string) (string, error) {
	s.logger.Debug(fmt.Sprintf("Validating token: %s", tokenString))
	// Используем AuthService для проверки токена
	userID, err := s.authService.ValidateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("token validation error: %w", err)
	}

	// Преобразуем ID пользователя в строку
	return fmt.Sprintf("%d", userID), nil
}

// checkUserConnectionLimit проверяет, не превышен ли лимит соединений для пользователя
func (s *Server) checkUserConnectionLimit(userID string) bool {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	sessions, exists := s.userSessions[userID]
	if !exists {
		return true
	}

	return len(sessions) < s.config.WebSocket.MaxConnectionsPerUser
}

// handleAudioMessage обрабатывает сообщение с аудиоданными
func (s *Server) handleAudioMessage(session *Session, message WebSocketMessage) {
	// Разбор аудиоданных
	var audioData AudioData
	if err := json.Unmarshal(message.Data, &audioData); err != nil {
		s.logger.Error(fmt.Sprintf("Error parsing audio data: %v", err))
		s.sendError(session, "DATA_ERROR", "Invalid audio data format")
		return
	}

	// Сохранение аудиоданных для последующей обработки
	session.mutex.Lock()
	session.AudioData = append(session.AudioData, audioData.Samples...)

	// Проверяем, начата ли сессия
	if !session.Started {
		// Если сессия не начата, считаем это началом сессии
		session.Started = true
		session.StartTime = time.Now() // Сбрасываем время начала

		// Инициализируем канал для ожидания проверки подписки
		// Буферизованный канал (1) позволит отправить результат, даже если никто не ждет
		session.subscriptionCheck = make(chan bool, 1)

		session.mutex.Unlock()

		s.logger.Info(fmt.Sprintf("Start recording session: %s for user: %s", session.ID, session.UserID))

		// Отправляем подтверждение начала сессии
		response := WebSocketMessage{
			Type:      MessageTypeResult,
			SessionID: session.ID,
			Data:      nil,
		}

		if err := s.sendMessage(session, response); err != nil {
			s.logger.Error(fmt.Sprintf("Error sending start confirmation: %v", err))
		}

		// Асинхронно проверяем подписку пользователя
		go func() {
			s.logger.Info(fmt.Sprintf("Checking subscription for user: %s", session.UserID))
			isValid, err := s.checkUserSubscription(session.UserID)

			if err != nil {
				s.logger.Error(fmt.Sprintf("Failed to check subscription: %v", err))
				// При ошибке проверки считаем, что подписки нет

				// Отправляем результат проверки в канал
				session.subscriptionCheck <- false

				// Отправляем сообщение об ошибке
				s.sendError(session, "SUBSCRIPTION_ERROR", "Failed to check subscription")

				// Закрываем соединение
				go s.closeSession(session)
				return
			}

			// Отправляем результат проверки в канал
			session.subscriptionCheck <- isValid

			if !isValid {
				s.logger.Warn(fmt.Sprintf("User has no valid subscription: %s", session.UserID))

				// Отправляем сообщение об ошибке
				s.sendError(session, "SUBSCRIPTION_ERROR", "No valid subscription")

				// Закрываем соединение
				go s.closeSession(session)
			} else {
				s.logger.Info(fmt.Sprintf("Valid subscription confirmed for user: %s", session.UserID))
				// Соединение остается открытым до получения сообщения stop
			}
		}()
	} else {
		session.mutex.Unlock()
	}
}

// handleStopMessage обрабатывает сообщение об окончании записи
func (s *Server) handleStopMessage(session *Session, _ WebSocketMessage) {
	// Блокируем сессию для проверки
	session.mutex.Lock()

	// Проверяем, была ли начата сессия
	if !session.Started {
		session.mutex.Unlock()
		s.logger.Warn(fmt.Sprintf("Attempt to stop not started session: %s for user: %s", session.ID, session.UserID))
		s.sendError(session, "SESSION_ERROR", "Session not started")
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	// Сохраняем ссылку на канал проверки подписки для дальнейшего использования
	subscriptionCheckChan := session.subscriptionCheck

	// Сохраняем аудиоданные и другую информацию
	audioData := session.AudioData
	session.AudioData = make([]byte, 0) // Очищаем аудиоданные
	session.Started = false             // Сбрасываем флаг начала сессии
	duration := time.Since(session.StartTime)

	// Разблокируем мьютекс, чтобы не блокировать другие операции во время ожидания
	session.mutex.Unlock()

	// Ждем результат проверки подписки
	s.logger.Info(fmt.Sprintf("Waiting for subscription check for session: %s", session.ID))

	var hasValidSubscription bool
	select {
	case isValid := <-subscriptionCheckChan:
		hasValidSubscription = isValid
	case <-time.After(5 * time.Second): // Таймаут ожидания проверки подписки
		s.logger.Error(fmt.Sprintf("Subscription check timeout for session: %s", session.ID))
		s.sendError(session, "SUBSCRIPTION_ERROR", "Subscription check timeout")
		go s.closeSession(session)
		return
	}

	// Проверяем валидность подписки
	if !hasValidSubscription {
		s.logger.Warn(fmt.Sprintf("Attempt to stop session without valid subscription: %s for user: %s", session.ID, session.UserID))
		s.sendError(session, "SUBSCRIPTION_ERROR", "No valid subscription")
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	s.logger.Info(fmt.Sprintf("Stop recording session: %s for user: %s", session.ID, session.UserID))

	// Обрабатываем собранные аудиоданные
	resultData, err := s.processAudioData(session.ID, session.UserID, audioData, duration)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio data: %v", err))
		s.sendError(session, "PROCESSING_ERROR", "Failed to process audio data")
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	// Отправляем результат обработки клиенту
	resultDataJSON, _ := json.Marshal(resultData)
	response := WebSocketMessage{
		Type:      MessageTypeResult,
		SessionID: session.ID,
		Data:      resultDataJSON,
	}

	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending result: %v", err))
	}

	// Закрываем соединение после отправки результата
	s.logger.Info(fmt.Sprintf("Closing connection after processing session: %s for user: %s", session.ID, session.UserID))
	go s.closeSession(session)
}

// sendError отправляет сообщение об ошибке клиенту
func (s *Server) sendError(session *Session, code string, message string) {
	errorData := ErrorData{
		Code:    code,
		Message: message,
	}

	errorDataJSON, _ := json.Marshal(errorData)
	response := WebSocketMessage{
		Type:      MessageTypeError,
		SessionID: session.ID,
		Data:      errorDataJSON,
	}

	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending error message: %v", err))
	}
}

// sendMessage отправляет сообщение клиенту
func (s *Server) sendMessage(session *Session, message WebSocketMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	session.mutex.Lock()
	defer session.mutex.Unlock()

	return session.Conn.WriteMessage(websocket.TextMessage, data)
}

// closeSession закрывает сессию и удаляет её из списка активных сессий
func (s *Server) closeSession(session *Session) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	// Закрываем соединение
	session.Conn.Close()

	// Удаляем сессию из списка
	delete(s.sessions, session.ID)

	// Удаляем сессию из списка сессий пользователя
	if sessions, exists := s.userSessions[session.UserID]; exists {
		delete(sessions, session.ID)
		if len(sessions) == 0 {
			delete(s.userSessions, session.UserID)
		}
	}

	s.logger.Info(fmt.Sprintf("Session closed: %s for user: %s", session.ID, session.UserID))
}

// checkUserSubscription проверяет наличие активной подписки у пользователя
// Возвращает true, если у пользователя есть активная подписка, и false в противном случае
// В текущей реализации это заглушка, которую нужно заменить на реальную проверку
func (s *Server) checkUserSubscription(userID string) (bool, error) {
	// Имитация задержки при проверке подписки
	time.Sleep(3000 * time.Millisecond)

	s.logger.Debug(fmt.Sprintf("Checking subscription for user: %s", userID))
	// FIXME: В реальной реализации здесь должен быть код для:
	// 1. Получения информации о пользователе из базы данных по userID
	// 2. Проверки наличия активной подписки в БД
	// 3. Проверки лимитов использования (например, количество минут в месяц)
	// 4. Возможно, обновления счетчиков использования

	// Заглушка: считаем, что у всех пользователей есть подписка
	return true, nil
}

// processAudioData обрабатывает собранные аудиоданные и возвращает результат распознавания
// В текущей реализации это заглушка, которую нужно заменить на реальную обработку
func (s *Server) processAudioData(sessionID string, userID string, audioData []byte, duration time.Duration) (ResultData, error) {
	// Имитация задержки при обработке аудио
	time.Sleep(300 * time.Millisecond)

	// Логируем информацию об обработке
	s.logger.Info(fmt.Sprintf("Processing %d bytes of audio data for session %s", len(audioData), sessionID))

	// FIXME: В реальной реализации здесь должен быть код для:
	// 1. Отправки аудиоданных в сторонний API для распознавания речи
	// 2. Ожидания и получения ответа от API
	// 3. Преобразования ответа в формат ResultData
	// 4. Обработки возможных ошибок от API

	// Заглушка: возвращаем фиксированный текст
	return ResultData{
		Text:     "Пример текста распознавания",
		Language: "ru",
		Duration: float64(duration.Seconds()),
	}, nil
}
