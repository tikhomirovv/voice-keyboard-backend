package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// Константы для работы с Realtime API
const (
	// WebSocket URL для Realtime API с intent=transcription
	realtimeAPIURL = "wss://api.openai.com/v1/realtime"

	// Типы событий от сервера к клиенту
	eventTypeTranscriptionDelta            = "conversation.item.input_audio_transcription.delta"
	eventTypeTranscriptionCompleted        = "conversation.item.input_audio_transcription.completed"
	eventTypeInputAudioBufferCommitted     = "conversation.item.input_audio_buffer.committed"
	eventTypeTranscriptionSessionCreated   = "transcription_session.created"
	eventTypeInputAudioBufferSpeechStarted = "input_audio_buffer.speech_started"
	eventTypeError                         = "error"

	// Типы событий от клиента к серверу
	eventTypeInputAudioBufferAppend = "input_audio_buffer.append"
	eventTypeInputAudioBufferCommit = "input_audio_buffer.commit"
	eventTypeTranscriptionSession   = "transcription_session.update"

	// Таймауты и интервалы
	sessionInactivityTimeout = 1 * time.Minute  // Таймаут неактивности сессии для очистки
	transcriptionWaitTimeout = 10 * time.Second // Таймаут ожидания завершения транскрипции (уменьшил с 30 до 10)
)

// RealtimeSession представляет сессию транскрипции с собственным подключением к OpenAI
type RealtimeSession struct {
	// Основная информация о сессии
	ID         string                      // Уникальный идентификатор сессии
	UserID     uint64                      // ID пользователя
	Format     string                      // Формат аудио (pcm16 и т.д.)
	Language   string                      // Язык транскрибации
	Prompt     string                      // Промпт для транскрибации
	ResultCh   chan *dto.TranscriberResult // Канал для отправки результатов транскрипции
	Created    time.Time                   // Время создания сессии
	LastActive time.Time                   // Время последней активности

	// Подключение к OpenAI
	conn      *websocket.Conn // WebSocket соединение
	config    *pkg.Config     // Конфигурация
	logger    logger.Logger   // Логгер
	closed    bool            // Флаг закрытия
	closeCh   chan struct{}   // Канал для сигнала закрытия
	connMutex sync.Mutex      // Мьютекс для соединения
	itemID    string          // Текущий ID элемента в разговоре
	lastText  string          // Последний полученный текст
	ready     bool            // Флаг готовности сессии
	isSpeech  bool            // Флаг, указывающий, идет ли речь

	// Упрощенное ожидание completed события
	completionCh chan struct{} // Канал для сигнала о получении completed события
}

// RealtimeEvent описывает структуру события от Realtime API
type RealtimeEvent struct {
	EventID        string         `json:"event_id,omitempty"`
	Type           string         `json:"type"`
	ItemID         string         `json:"item_id,omitempty"`
	ContentIndex   int            `json:"content_index,omitempty"`
	Delta          string         `json:"delta,omitempty"`
	Transcript     string         `json:"transcript,omitempty"`
	PreviousItemID string         `json:"previous_item_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Error          *RealtimeError `json:"error,omitempty"`
}

// RealtimeError описывает ошибку от Realtime API
type RealtimeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

// AudioBufferAppendEvent описывает событие для отправки аудиоданных
type AudioBufferAppendEvent struct {
	Type     string         `json:"type"`
	Audio    string         `json:"audio"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AudioBufferCommitEvent описывает событие для коммита аудиобуфера
type AudioBufferCommitEvent struct {
	Type string `json:"type"`
}

// TranscriptionSessionUpdateEvent описывает событие для обновления сессии транскрипции
type TranscriptionSessionUpdateEvent struct {
	Type    string        `json:"type"`
	Session SessionObject `json:"session"`
}

type SessionObject struct {
	InputAudioFormat         string                   `json:"input_audio_format"`
	InputAudioTranscription  *InputAudioTranscription `json:"input_audio_transcription"`
	TurnDetection            *TurnDetection           `json:"turn_detection"`
	InputAudioNoiseReduction *NoiseReduction          `json:"input_audio_noise_reduction"`
	Include                  []string                 `json:"include,omitempty"`
}

// InputAudioTranscription описывает настройки транскрипции
type InputAudioTranscription struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt,omitempty"`
	Language string `json:"language,omitempty"`
}

// TurnDetection описывает настройки обнаружения речи
type TurnDetection struct {
	Type string `json:"type"`
	// Threshold         float64 `json:"threshold"`
	// PrefixPaddingMS   int     `json:"prefix_padding_ms"`
	// SilenceDurationMS int     `json:"silence_duration_ms"`
	Eagerness string `json:"eagerness"` // (semantic_vad) "low" | "medium" | "high" | "auto", // optional
}

// NoiseReduction описывает настройки шумоподавления
type NoiseReduction struct {
	Type string `json:"type"`
}

// RealtimeTranscriberService реализует интерфейс RealtimeTranscriberServiceInterface
// с использованием OpenAI Realtime API для транскрипции аудио в реальном времени
type RealtimeTranscriberService struct {
	config        *pkg.Config
	logger        logger.Logger
	sessionsMutex sync.RWMutex
	sessions      map[string]*RealtimeSession
}

// NewRealtimeSession создает новую сессию для работы с OpenAI Realtime API
func NewRealtimeSession(
	config *pkg.Config,
	logger logger.Logger,
	sessionID string,
	format string,
	language string,
) *RealtimeSession {
	resultCh := make(chan *dto.TranscriberResult, 10)

	return &RealtimeSession{
		ID:           sessionID,
		Format:       format,
		Language:     language,
		ResultCh:     resultCh,
		Created:      time.Now(),
		LastActive:   time.Now(),
		config:       config,
		logger:       logger,
		closed:       false,
		closeCh:      make(chan struct{}),
		ready:        false,
		completionCh: make(chan struct{}),
	}
}

// Connect устанавливает соединение сессии с OpenAI Realtime API
func (s *RealtimeSession) Connect() error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.ready {
		return nil // Уже подключены
	}

	// Формируем URL с параметром intent=transcription
	u, err := url.Parse(realtimeAPIURL)
	if err != nil {
		return fmt.Errorf("failed to parse Realtime API URL: %w", err)
	}

	q := u.Query()
	q.Set("intent", "transcription")
	u.RawQuery = q.Encode()

	// Заголовки для подключения
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+s.config.OpenAI.APIKey)
	headers.Set("OpenAI-Beta", "realtime=v1")

	// Подключаемся к WebSocket серверу
	s.logger.Info(fmt.Sprintf("Session %s: Connecting to OpenAI Realtime API", s.ID))
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return fmt.Errorf("failed to connect to Realtime API: %w", err)
	}

	s.conn = conn
	s.closed = false

	// Инициализируем сессию транскрипции
	if err := s.initTranscriptionSession(); err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to initialize transcription session: %w", err)
	}

	// Запускаем обработчик сообщений
	go s.handleMessages()

	s.ready = true
	s.logger.Info(fmt.Sprintf("Session %s: Successfully connected to OpenAI Realtime API", s.ID))
	return nil
}

// Close закрывает соединение с OpenAI Realtime API
func (s *RealtimeSession) Close() error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	close(s.closeCh)
	close(s.ResultCh)

	if s.conn != nil {
		return s.conn.Close()
	}

	return nil
}

// initTranscriptionSession инициализирует сессию транскрипции
func (s *RealtimeSession) initTranscriptionSession() error {
	// Настройки сессии транскрипции
	sessionUpdate := TranscriptionSessionUpdateEvent{
		Type: eventTypeTranscriptionSession,
		Session: SessionObject{
			InputAudioFormat: s.Format, // Используем формат из параметров сессии
			InputAudioTranscription: &InputAudioTranscription{
				Model:    "gpt-4o-mini-transcribe", // Используем модель gpt-4o-mini-transcribe
				Prompt:   s.Prompt,                 // Используем промпт из параметров сессии
				Language: s.Language,               // Используем язык из параметров сессии
			},
			TurnDetection: &TurnDetection{
				// Type:              "server_vad",
				Type:      "semantic_vad",
				Eagerness: "low",
				// Threshold:         0.5,
				// PrefixPaddingMS:   300,
				// SilenceDurationMS: 500,
			},
			InputAudioNoiseReduction: &NoiseReduction{
				Type: "near_field", // Шумоподавление для близкого источника
			},
		},
	}

	// Отправляем событие инициализации сессии
	return s.sendJSON(sessionUpdate)
}

// handleMessages обрабатывает входящие сообщения от OpenAI Realtime API
func (s *RealtimeSession) handleMessages() {
	defer func() {
		s.logger.Info(fmt.Sprintf("Session %s: Exiting OpenAI Realtime API message handler", s.ID))
	}()

	for {
		select {
		case <-s.closeCh:
			return
		default:
			// Читаем сообщение
			_, message, err := s.conn.ReadMessage()
			if err != nil {
				// Проверяем, не закрыт ли канал (что означает нормальное завершение сессии)
				select {
				case <-s.closeCh:
					// Канал закрыт, это нормальное завершение работы
					s.logger.Info(fmt.Sprintf("Session %s: Connection closed normally", s.ID))
				default:
					// Канал не закрыт, это реальная ошибка чтения
					s.logger.Error(fmt.Sprintf("Session %s: Error reading from Realtime API: %v", s.ID, err))
				}
				return
			}

			// Разбираем JSON
			var event RealtimeEvent
			if err := json.Unmarshal(message, &event); err != nil {
				s.logger.Error(fmt.Sprintf("Session %s: Error parsing Realtime API message: %v", s.ID, err))
				continue
			}

			// Обрабатываем событие в зависимости от типа
			switch event.Type {
			case eventTypeTranscriptionDelta:
				s.handleTranscriptionDelta(&event)
			case eventTypeTranscriptionCompleted:
				s.handleTranscriptionCompleted(&event)
			case eventTypeInputAudioBufferCommitted:
				s.handleAudioBufferCommitted(&event)
			case eventTypeInputAudioBufferSpeechStarted:
				s.handleAudioBufferSpeechStarted(&event)
			case eventTypeError:
				s.logger.Error(fmt.Sprintf("Session %s: Received error from Realtime API: %+v", s.ID, event.Error))
			case eventTypeTranscriptionSessionCreated:
				// Добавляем обработку события создания сессии транскрипции
				s.logger.Info(fmt.Sprintf("Session %s: Transcription session successfully created", s.ID))
				if len(event.Metadata) > 0 {
					metadataJSON, _ := json.Marshal(event.Metadata)
					s.logger.Debug(fmt.Sprintf("Session %s: Session creation metadata: %s", s.ID, string(metadataJSON)))
				}
			default:
				s.logger.Debug(fmt.Sprintf("Session %s: Received unhandled event type: %s", s.ID, event.Type))
			}
		}
	}
}

// handleTranscriptionDelta обрабатывает промежуточные результаты транскрипции
func (s *RealtimeSession) handleTranscriptionDelta(event *RealtimeEvent) {
	// Обновляем последний текст
	// s.lastText += event.Delta
	s.logger.Debug(fmt.Sprintf("Session %s: Delta transcript: %s", s.ID, event.Delta))
}

func (s *RealtimeSession) handleAudioBufferSpeechStarted(_ *RealtimeEvent) {
	s.logger.Debug(fmt.Sprintf("Session %s: Audio buffer speech started", s.ID))
	s.isSpeech = true
}

// handleTranscriptionCompleted обрабатывает завершенные результаты транскрипции
func (s *RealtimeSession) handleTranscriptionCompleted(event *RealtimeEvent) {
	s.lastText += event.Transcript

	// Отправляем результат в канал
	result := &dto.TranscriberResult{
		Text: event.Transcript,
	}

	select {
	case s.ResultCh <- result:
		s.logger.Info(fmt.Sprintf("Session %s: Sent transcript: %s", s.ID, event.Transcript))
	default:
		s.logger.Warn(fmt.Sprintf("Session %s: Failed to send transcript: channel full or closed", s.ID))
	}

	// Отправляем сигнал о получении completed события
	select {
	case s.completionCh <- struct{}{}:
		s.logger.Debug(fmt.Sprintf("Session %s: Sent completion signal", s.ID))
	default:
		// Неблокирующая отправка - если никто не ждет, просто игнорируем
	}
}

// handleAudioBufferCommitted обрабатывает события завершения буфера аудио
func (s *RealtimeSession) handleAudioBufferCommitted(event *RealtimeEvent) {
	s.logger.Debug(fmt.Sprintf("Session %s: Audio buffer committed: item_id=%s, previous_item_id=%s",
		s.ID, event.ItemID, event.PreviousItemID))

	// Обновляем ItemID в сессии
	s.itemID = event.ItemID
}

// sendAudioData отправляет аудиоданные для сессии
func (s *RealtimeSession) sendAudioData(audioData string) error {
	// Проверяем активность сессии
	if s.closed {
		return fmt.Errorf("session is closed")
	}

	// Обновляем время последней активности
	s.LastActive = time.Now()

	// Отправляем аудиоданные
	event := AudioBufferAppendEvent{
		Type:  eventTypeInputAudioBufferAppend,
		Audio: audioData,
	}

	if err := s.sendJSON(event); err != nil {
		return fmt.Errorf("failed to send audio data: %w", err)
	}

	return nil
}

// commitAudioBuffer отправляет запрос на коммит аудиобуфера
func (s *RealtimeSession) commitAudioBuffer() error {
	// Проверяем активность сессии
	if s.closed {
		return fmt.Errorf("session is closed")
	}

	// Обновляем время последней активности
	s.LastActive = time.Now()

	// Отправляем запрос на коммит
	event := AudioBufferCommitEvent{
		Type: eventTypeInputAudioBufferCommit,
	}

	if err := s.sendJSON(event); err != nil {
		return fmt.Errorf("failed to commit audio buffer: %w", err)
	}

	s.logger.Info(fmt.Sprintf("Session %s: Sent audio buffer commit request", s.ID))
	return nil
}

// sendJSON отправляет JSON объект в WebSocket соединение
func (s *RealtimeSession) sendJSON(v any) error {
	if s.closed || s.conn == nil {
		return fmt.Errorf("session is closed or not connected")
	}

	return s.conn.WriteJSON(v)
}

// GetLastText возвращает последний полученный текст транскрипции
func (s *RealtimeSession) GetLastText() string {
	return s.lastText
}

// WaitForCompletion ожидает следующего completed события с таймаутом
func (s *RealtimeSession) WaitForCompletion(ctx context.Context) bool {
	select {
	case <-s.completionCh:
		return true
	case <-time.After(transcriptionWaitTimeout):
		return false
	case <-ctx.Done():
		return false
	case <-s.closeCh:
		return false
	}
}

// NewRealtimeTranscriberService создает новый сервис транскрипции файлов с использованием OpenAI Realtime API
func NewRealtimeTranscriberService(
	config *pkg.Config,
	logger logger.Logger,
) *RealtimeTranscriberService {
	service := &RealtimeTranscriberService{
		config:   config,
		logger:   logger,
		sessions: make(map[string]*RealtimeSession),
	}

	// Запускаем сервис очистки устаревших сессий
	go service.cleanSessions()

	return service
}

// cleanSessions периодически очищает устаревшие сессии
func (s *RealtimeTranscriberService) cleanSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		var sessionsToClose []string

		s.sessionsMutex.RLock()
		for id, session := range s.sessions {
			// Закрываем сессию, если она неактивна более sessionInactivityTimeout
			if now.Sub(session.LastActive) > sessionInactivityTimeout {
				sessionsToClose = append(sessionsToClose, id)
			}
		}
		s.sessionsMutex.RUnlock()

		// Закрываем устаревшие сессии
		for _, id := range sessionsToClose {
			s.logger.Info(fmt.Sprintf("Cleaning inactive session %s due to inactivity timeout (%v)", id, sessionInactivityTimeout))
			// Используем контекст с таймаутом для закрытия
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.CloseSession(ctx, id)
			cancel()
		}
	}
}

// StartSession начинает новую сессию транскрибации в реальном времени
func (s *RealtimeTranscriberService) StartSession(
	ctx context.Context,
	options *interfaces.RealtimeSessionOptions,
) (string, <-chan *dto.TranscriberResult, error) {

	// Создаем новую сессию с собственным подключением
	session := NewRealtimeSession(
		s.config,
		s.logger,
		options.SessionID,
		options.Format,
		options.Language,
		// options.Prompt,
	)

	// Устанавливаем соединение с OpenAI
	if err := session.Connect(); err != nil {
		return "", nil, fmt.Errorf("failed to connect session to OpenAI: %w", err)
	}

	// Регистрируем сессию
	s.sessionsMutex.Lock()
	s.sessions[session.ID] = session
	s.sessionsMutex.Unlock()

	s.logger.Info(fmt.Sprintf("Started new realtime transcription session: %s", session.ID))

	return session.ID, session.ResultCh, nil
}

// AppendAudio добавляет аудиоданные в текущую сессию
func (s *RealtimeTranscriberService) AppendAudio(ctx context.Context, sessionID string, audioData string) error {
	// Находим сессию
	s.sessionsMutex.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Отправляем аудиоданные через сессию
	if err := session.sendAudioData(audioData); err != nil {
		s.logger.Error(fmt.Sprintf("Failed to send audio data for session %s: %v", sessionID, err))
		return fmt.Errorf("failed to send audio data: %w", err)
	}

	return nil
}

// CompleteSession завершает сессию транскрибации и возвращает финальный результат
func (s *RealtimeTranscriberService) CompleteSession(ctx context.Context, sessionID string) (*dto.TranscriberResult, error) {
	// Находим сессию
	s.sessionsMutex.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Пытаемся коммитить буфер (может быть пустым - это нормально)
	if err := session.commitAudioBuffer(); err != nil {
		s.logger.Warn(fmt.Sprintf("Session %s: Failed to commit audio buffer: %v", sessionID, err))
		// Продолжаем выполнение, поскольку это не критическая ошибка
	}

	// Ждем следующего completed события или таймаута
	completedSuccessfully := session.WaitForCompletion(ctx)

	if !completedSuccessfully {
		if ctx.Err() != nil {
			s.logger.Warn(fmt.Sprintf("Session %s: Context cancelled while waiting for completion", sessionID))
		} else {
			s.logger.Warn(fmt.Sprintf("Session %s: Timeout waiting for next completion event after %v",
				sessionID, transcriptionWaitTimeout))
		}
	} else {
		s.logger.Info(fmt.Sprintf("Session %s: Received completion event after request", sessionID))
	}

	// Создаем результат на основе последней информации
	finalResult := &dto.TranscriberResult{
		Text: session.GetLastText(),
	}

	// Закрываем сессию и очищаем ресурсы
	s.CloseSession(ctx, sessionID)

	return finalResult, nil
}

// CloseSession закрывает сессию транскрибации и освобождает ресурсы
func (s *RealtimeTranscriberService) CloseSession(ctx context.Context, sessionID string) error {
	s.sessionsMutex.Lock()
	session, exists := s.sessions[sessionID]
	if exists {
		delete(s.sessions, sessionID)
	}
	s.sessionsMutex.Unlock()

	if !exists {
		return nil // Сессия уже была удалена
	}

	// Закрываем соединение и ресурсы сессии
	if err := session.Close(); err != nil {
		s.logger.Warn(fmt.Sprintf("Error closing session %s: %v", sessionID, err))
	}

	s.logger.Info(fmt.Sprintf("Cancelled realtime transcription session: %s", sessionID))

	return nil
}

// Close освобождает ресурсы сервиса и закрывает все активные сессии
func (s *RealtimeTranscriberService) Close() error {
	s.sessionsMutex.Lock()
	sessions := make([]*RealtimeSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.sessions = make(map[string]*RealtimeSession)
	s.sessionsMutex.Unlock()

	// Закрываем каждую сессию отдельно
	for _, session := range sessions {
		if err := session.Close(); err != nil {
			s.logger.Warn(fmt.Sprintf("Error closing session %s: %v", session.ID, err))
		}
	}

	s.logger.Info("Successfully closed all realtime transcription sessions")

	return nil
}
