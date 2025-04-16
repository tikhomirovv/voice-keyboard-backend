package websocket

/*
WebSocket Protocol

Подключение:
1. Клиент должен подключаться к WebSocket-серверу с двумя обязательными параметрами:
   - Authorization заголовок HTTP вида "Bearer <jwt-token>"
   - URL-параметр sessionId, уникальный идентификатор сессии

   Пример URL для подключения:
   ws://server:port/path?sessionId=abc123

2. После успешного подключения клиент может отправлять сообщения двух типов:
   - audio: содержит аудиоданные для обработки
   - stop: сигнализирует о завершении отправки аудиоданных

3. Сервер может отправлять сообщения двух типов:
   - result: содержит результат обработки аудиоданных
   - error: содержит информацию об ошибке

Проверка подписки и управление сессией:
1. При подключении клиента создается сессия и асинхронно запускается проверка подписки
2. Если проверка подписки выдает ошибку, соединение закрывается немедленно
3. Результат проверки (успешный/неуспешный) сохраняется в канале сессии
4. При получении команды "stop" проверяется наличие подписки по результату из канала
5. Если подписки нет, сессия закрывается без обработки аудио
6. Закрытие сессии (closeSession) автоматически освобождает все ресурсы, включая файлы

Все сообщения должны соответствовать формату WebSocketMessage.
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// MessageType определяет тип сообщения, которое отправляется через WebSocket
type MessageType string

const (
	// Типы сообщений от клиента к серверу
	MessageTypeAudio MessageType = "audio" // Аудиоданные
	MessageTypeStop  MessageType = "stop"  // Конец сессии

	// Типы сообщений от сервера к клиенту
	MessageTypePartial   MessageType = "partial"   // Частичный результат транскрипции
	MessageTypeCompleted MessageType = "completed" // Результат транскрипции
	MessageTypeError     MessageType = "error"     // Ошибка
)

// WebSocketMessage представляет структуру сообщения через WebSocket
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	Data      json.RawMessage `json:"data"`
}

// AudioData содержит аудиоданные в сообщении
type AudioData struct {
	Samples string `json:"samples"` // Байты аудиосэмплов в base64
}

// CompletedData содержит результат распознавания
type CompletedData struct {
	Text string `json:"text"`
}

// PartialData содержит частичное распознанное текстовое сообщение
type PartialData struct {
	Text string `json:"text"`
}

// ErrorData содержит информацию об ошибке
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Session представляет активную сессию WebSocket
type Session struct {
	ID                string
	UserID            uint64
	SampleRate        uint32
	Format            string
	Conn              *websocket.Conn
	StartTime         time.Time
	LastTime          time.Time
	AudioFilePath     string    // Путь к временному файлу с аудиоданными
	AudioFile         *os.File  // Дескриптор файла для записи аудиоданных
	Started           bool      // Флаг, указывающий, была ли сессия начата
	subscriptionCheck chan bool // Канал для ожидания завершения проверки подписки
	mutex             sync.Mutex
}

// Server представляет WebSocket-сервер для аудиостриминга
type Server struct {
	config       *pkg.Config
	logger       logger.Logger
	authService  interfaces.AuthServiceInterface
	audioService interfaces.AudioServiceInterface // Сервис для работы с аудиоданными
	upgrader     websocket.Upgrader
	sessions     map[string]*Session
	userSessions map[uint64]map[string]bool // UserID -> map[SessionID]bool
	sessionMutex sync.RWMutex
	httpServer   *http.Server
	vl           *validator.Validate
}

// NewServer создает новый WebSocket-сервер
func NewServer(c *pkg.Container) *Server {
	return &Server{
		config:       c.Config,
		logger:       c.Logger,
		authService:  c.AuthService,
		audioService: c.AudioService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Здесь может быть проверка origin
				return true
			},
		},
		sessions:     make(map[string]*Session),
		userSessions: make(map[uint64]map[string]bool),
		vl:           c.Validator,
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

		// Закрываем все активные соединения корректно
		s.sessionMutex.Lock()
		s.logger.Info("Gracefully closing all active WebSocket connections")
		for _, session := range s.sessions {
			if session.Conn != nil {
				s.gracefulCloseConn(session.Conn)
			}
		}
		s.sessions = make(map[string]*Session)
		s.userSessions = make(map[uint64]map[string]bool)
		s.sessionMutex.Unlock()

		// Останавливаем HTTP-сервер
		s.logger.Info("Stopping WebSocket server")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// startSubscriptionCheck запускает асинхронную проверку подписки пользователя
// и отправляет результат в канал session.subscriptionCheck
func (s *Server) startSubscriptionCheck(session *Session) {
	go func() {
		s.logger.Info(fmt.Sprintf("Checking subscription for user: %d", session.UserID))
		isValid, err := s.checkUserSubscription(session.UserID)

		if err != nil {
			s.logger.Error(fmt.Sprintf("Failed to check subscription: %v", err))
			// При ошибке проверки считаем, что подписки нет

			// Отправляем сообщение об ошибке
			s.sendError(session, "SUBSCRIPTION_ERROR", "Failed to check subscription")

			// Отправляем результат проверки в канал
			session.subscriptionCheck <- false

			// Закрываем соединение только при ошибке проверки
			go s.closeSession(session)
			return
		}

		// Отправляем результат проверки в канал
		session.subscriptionCheck <- isValid

		if !isValid {
			s.logger.Warn(fmt.Sprintf("User has no valid subscription: %d", session.UserID))
			s.sendError(session, "SUBSCRIPTION_ERROR", "User has no valid subscription")
			go s.closeSession(session)

		} else {
			s.logger.Info(fmt.Sprintf("Valid subscription confirmed for user: %d", session.UserID))
		}
	}()
}

// initSession инициализирует новую WebSocket сессию
// Возвращает созданную сессию и ошибку, если что-то пошло не так
func (s *Server) initSession(userID uint64, sessionID string, sampleRate uint32, format string, conn *websocket.Conn) (*Session, error) {
	// Создание новой сессии
	session := &Session{
		ID:                sessionID,
		UserID:            userID,
		SampleRate:        sampleRate,
		Format:            format,
		Conn:              conn,
		StartTime:         time.Now(),
		LastTime:          time.Now(),
		AudioFilePath:     "",
		AudioFile:         nil,
		Started:           false, // Инициализируем флаг сессии как не начатую
		subscriptionCheck: make(chan bool, 1),
		mutex:             sync.Mutex{},
	}

	// Регистрируем сессию в обработчике под блокировкой
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	// Регистрируем сессию в картах сервера
	s.sessions[sessionID] = session

	// Добавляем сессию в список сессий пользователя
	if s.userSessions[session.UserID] == nil {
		s.userSessions[session.UserID] = make(map[string]bool)
	}
	s.userSessions[session.UserID][sessionID] = true

	s.logger.Info(fmt.Sprintf("Session successfully initialized with ID: %s for user: %d", sessionID, userID))

	// Асинхронно проверяем подписку пользователя сразу при создании сессии
	go s.startSubscriptionCheck(session)

	return session, nil
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
	session, err := s.initSession(userID, wsConnectAudioDTO.SessionID, wsConnectAudioDTO.SampleRate, wsConnectAudioDTO.Format, conn)
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

// handleSession обрабатывает все сообщения WebSocket в рамках одной сессии
func (s *Server) handleSession(session *Session) {
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
		session.LastTime = time.Now()
		session.Conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))
		session.Conn.SetWriteDeadline(time.Now().Add(time.Second * time.Duration(s.config.WebSocket.IdleTimeout)))

		// Разбор сообщения
		var wsMessage WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			s.logger.Error(fmt.Sprintf("Error parsing message: %v", err))
			s.sendError(session, "PARSE_ERROR", "Invalid message format")
			continue
		}

		// Проверяем соответствие ID сессии
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
			// Для стоп-сообщения завершаем обработку после его выполнения
			return
		default:
			s.logger.Warn(fmt.Sprintf("Unknown message type: %s", wsMessage.Type))
			s.sendError(session, "UNKNOWN_TYPE", "Unknown message type")
		}
	}
}

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

// handleAudioMessage обрабатывает сообщение с аудиоданными
func (s *Server) handleAudioMessage(session *Session, message WebSocketMessage) {
	// Разбор аудиоданных
	var audioData AudioData
	if err := json.Unmarshal(message.Data, &audioData); err != nil {
		s.logger.Error(fmt.Sprintf("Error parsing audio data: %v", err))
		s.sendError(session, "DATA_ERROR", "Invalid audio data format")
		return
	}

	// Добавляем дополнительное логирование для отладки
	// s.logger.Debug(fmt.Sprintf("Received audio data, samples size: %d bytes",
	// len(audioData.Samples)))

	// Проверяем, не пусты ли данные
	if len(audioData.Samples) == 0 {
		s.logger.Error("Empty audio data received")
		s.sendError(session, "DATA_ERROR", "Empty audio data")
		return
	}

	// Декодируем данные из Base64
	rawAudioBytes, err := base64.StdEncoding.DecodeString(audioData.Samples)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error decoding base64 audio data: %v", err))
		s.sendError(session, "DATA_ERROR", "Error decoding base64 audio data")
		return
	}

	// Создаем структуру с форматом аудио
	format := &AudioFormat{
		Format:     session.Format,
		SampleRate: session.SampleRate,
	}

	// Получаем процессор для указанного формата
	processor, err := ValidateAndGetProcessor(format)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error with audio format: %v", err))
		s.sendError(session, "FORMAT_ERROR", err.Error())
		return
	}
	// Обработка аудиоданных с соответствующим процессором
	processedData, err := processor.Process(rawAudioBytes)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio: %v", err))
		s.sendError(session, "PROCESSING_ERROR", "Failed to process audio data")
		return
	}

	// s.logger.Debug(fmt.Sprintf("Successfully decoded %d bytes of audio data", len(rawAudioBytes)))

	session.mutex.Lock()

	// Проверяем, начата ли сессия
	if !session.Started {
		// Если сессия не начата, считаем это началом сессии
		session.Started = true
		session.StartTime = time.Now() // Сбрасываем время начала

		// Создаем файл для сохранения аудиоданных
		audioFilePath, err := s.audioService.Create(session.UserID, session.ID, session.SampleRate)
		if err != nil {
			session.mutex.Unlock()
			s.logger.Error(fmt.Sprintf("Failed to create audio file: %v", err))
			s.sendError(session, "INTERNAL_ERROR", "Failed to prepare for recording")
			return
		}
		session.AudioFilePath = audioFilePath
	}

	// Записываем данные в файл
	if err := s.audioService.WriteData(session.ID, processedData); err != nil {
		session.mutex.Unlock()
		s.logger.Error(fmt.Sprintf("Error writing to audio file: %v", err))
		s.sendError(session, "INTERNAL_ERROR", "Failed to save audio data")
		return
	}
	session.mutex.Unlock()
}

// waitForSubscriptionResult ожидает результат проверки подписки с таймаутом
// Возвращает true, если у пользователя есть действительная подписка, false в противном случае
// Если возникла ошибка (таймаут или отсутствие подписки), функция отправляет сообщение об ошибке,
// но НЕ закрывает соединение (это делает вызывающий код)
func (s *Server) waitForSubscriptionResult(session *Session) bool {
	// Проверяем результат проверки подписки, которая была запущена при инициализации сессии
	s.logger.Info(fmt.Sprintf("Checking subscription result for session: %s", session.ID))

	select {
	case isValid := <-session.subscriptionCheck:
		if !isValid {
			s.logger.Warn(fmt.Sprintf("Attempt to process audio with invalid subscription: %s for user: %d", session.ID, session.UserID))
			s.sendError(session, "SUBSCRIPTION_ERROR", "Valid subscription required to process audio")
			return false
		}
		return true

	case <-time.After(5 * time.Second): // Таймаут ожидания проверки подписки
		s.logger.Error(fmt.Sprintf("Subscription check timeout for session: %s", session.ID))
		s.sendError(session, "SUBSCRIPTION_ERROR", "Subscription check timeout")
		return false
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

	// Закрываем файл и сохраняем путь для дальнейшей обработки
	audioFilePath := session.AudioFilePath
	if session.AudioFile != nil {
		// Синхронизируем данные с диском и закрываем файл через audioService
		if _, err := s.audioService.Close(session.ID); err != nil {
			s.logger.Error(fmt.Sprintf("Error closing audio file: %v", err))
		}
		session.AudioFile = nil
	}

	session.Started = false // Сбрасываем флаг начала сессии
	duration := time.Since(session.StartTime)

	// Разблокируем мьютекс, чтобы не блокировать другие операции во время ожидания
	session.mutex.Unlock()

	// Проверяем подписку пользователя
	if !s.waitForSubscriptionResult(session) {
		// Подписка недействительна или произошел таймаут, закрываем соединение
		go s.closeSession(session)
		return
	}

	s.logger.Info(fmt.Sprintf("Stop recording session: %s for user: %s", session.ID, session.UserID))

	// Обрабатываем собранные аудиоданные из файла
	result, err := s.processAudioDataFromFile(session.ID, session.UserID, audioFilePath, duration)

	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio data: %v", err))
		s.sendError(session, "PROCESSING_ERROR", "Failed to process audio data")
		// Закрываем соединение после ошибки
		go s.closeSession(session)
		return
	}

	// Отправляем результат обработки клиенту
	completedData := CompletedData{
		Text: result,
	}
	completedDataJSON, _ := json.Marshal(completedData)
	response := WebSocketMessage{
		Type:      MessageTypeCompleted,
		SessionID: session.ID,
		Data:      completedDataJSON,
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

// gracefulCloseConn корректно закрывает WebSocket соединение в соответствии с протоколом
// Отправляет Close фрейм и ждет подтверждения от клиента с таймаутом
func (s *Server) gracefulCloseConn(conn *websocket.Conn) {
	// Устанавливаем дедлайн для закрытия соединения
	deadline := time.Now().Add(2 * time.Second)
	if err := conn.SetWriteDeadline(deadline); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to set write deadline for close: %v", err))
	}

	// Отправляем фрейм Close с кодом и сообщением
	// 1000 - нормальное закрытие, "closing" - причина закрытия
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing")
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

// closeSession закрывает сессию и удаляет её из списка активных сессий
func (s *Server) closeSession(session *Session) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	// Проверяем, не была ли сессия уже закрыта
	_, exists := s.sessions[session.ID]
	if !exists {
		// Сессия уже была закрыта, ничего не делаем
		return
	}

	if _, err := s.audioService.Close(session.ID); err != nil {
		s.logger.Warn(fmt.Sprintf("Error closing audio file: %v", err))
	}

	// Удаляем временный файл, если он существует, через audioService
	// if err := s.audioService.Remove(session.UserID, session.ID); err != nil {
	// 	s.logger.Warn(fmt.Sprintf("Failed to delete temporary audio file: %v", err))
	// }

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

// checkUserSubscription проверяет наличие активной подписки у пользователя
// Возвращает true, если у пользователя есть активная подписка, и false в противном случае
// В текущей реализации это заглушка, которую нужно заменить на реальную проверку
func (s *Server) checkUserSubscription(userID uint64) (bool, error) {
	s.logger.Debug(fmt.Sprintf("Checking subscription for user: %d", userID))
	// Имитация задержки при проверке подписки
	time.Sleep(1000 * time.Millisecond)

	// FIXME: В реальной реализации здесь должен быть код для:
	// 1. Получения информации о пользователе из базы данных по userID
	// 2. Проверки наличия активной подписки в БД
	// 3. Проверки лимитов использования (например, количество минут в месяц)
	// 4. Возможно, обновления счетчиков использования

	// Заглушка: считаем, что у всех пользователей есть подписка
	// return true, nil
	return true, nil
}

// processAudioDataFromFile обрабатывает собранные аудиоданные из файла и возвращает результат распознавания
func (s *Server) processAudioDataFromFile(
	sessionID string,
	userID uint64,
	filePath string,
	duration time.Duration) (string, error) {
	// Обрабатываем аудиоданные из файла
	// Примечание: удаление файла выполняется в closeSession, а не здесь

	// // Используем audioService для обработки файла с аудиоданными
	// result, err := s.audioService.ProcessAudioData(sessionID, userID, filePath, duration)
	// if err != nil {
	// 	return ResultData{}, fmt.Errorf("error processing audio data: %w", err)
	// }

	// Заглушка: возвращаем фиксированный результат
	return "Пример текста распознавания (из файла)", nil
}
