package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

type Session struct {
	id string
	// Поля для работы с реалтайм транскрипцией
	resultCh           <-chan *dto.TranscriberResult // Канал для получения результатов транскрипции
	lastTranscriptText string                        // Последний полученный текст транскрипции
	callback           func(text string)
	mutex              sync.RWMutex // Мьютекс для защиты конкурентного доступа
}

// GetID возвращает ID сессии (безопасно, так как id не изменяется)
func (s *Session) GetID() string {
	return s.id
}

// SetLastTranscriptText безопасно устанавливает последний текст транскрипции
func (s *Session) SetLastTranscriptText(text string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.lastTranscriptText = text
}

// GetLastTranscriptText безопасно возвращает последний текст транскрипции
func (s *Session) GetLastTranscriptText() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastTranscriptText
}

// SetResultChannel безопасно устанавливает канал результатов
func (s *Session) SetResultChannel(ch <-chan *dto.TranscriberResult) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.resultCh = ch
}

// GetResultChannel безопасно возвращает канал результатов
func (s *Session) GetResultChannel() <-chan *dto.TranscriberResult {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.resultCh
}

// SetCallback безопасно устанавливает callback функцию
func (s *Session) SetCallback(callback func(text string)) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.callback = callback
}

// GetCallback безопасно возвращает callback функцию
func (s *Session) GetCallback() func(text string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.callback
}

// CallCallback безопасно вызывает callback функцию с проверкой на nil
func (s *Session) CallCallback(text string) {
	s.mutex.RLock()
	callback := s.callback
	s.mutex.RUnlock()

	if callback != nil {
		callback(text)
	}
}

// IsActive проверяет, активна ли сессия (есть ли канал результатов)
func (s *Session) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.resultCh != nil
}

// RealtimeWebSocketService реализует обработку WebSocket сообщений в реальном времени
type RealtimeWebSocketService struct {
	logger                logger.Logger
	transcriberService    interfaces.RealtimeTranscriberServiceInterface
	textGenerationService interfaces.LLMTextGenerationServiceInterface
	sessions              map[string]*Session
	sessionsMutex         sync.RWMutex // Мьютекс для защиты map sessions
}

// NewRealtimeWebSocketService создает новый сервис для обработки WebSocket в реальном времени
func NewRealtimeWebSocketService(
	logger logger.Logger,
	transcriberService interfaces.RealtimeTranscriberServiceInterface,
	textGenerationService interfaces.LLMTextGenerationServiceInterface,
) *RealtimeWebSocketService {
	return &RealtimeWebSocketService{
		logger:                logger,
		transcriberService:    transcriberService,
		textGenerationService: textGenerationService,
		sessions:              make(map[string]*Session),
	}
}

func (s *RealtimeWebSocketService) createSession(sessionID string, callback func(result string)) *Session {
	session := &Session{
		id:                 sessionID,
		lastTranscriptText: "",
		callback:           callback,
	}

	s.AddSession(session)
	return session
}

// AddSession безопасно добавляет сессию в map
func (s *RealtimeWebSocketService) AddSession(session *Session) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	s.sessions[session.id] = session
}

// GetSession безопасно возвращает сессию по ID
func (s *RealtimeWebSocketService) GetSession(sessionID string) *Session {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	return s.sessions[sessionID]
}

// RemoveSession безопасно удаляет сессию из map
func (s *RealtimeWebSocketService) RemoveSession(sessionID string) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	delete(s.sessions, sessionID)
}

// HasSession проверяет существование сессии
func (s *RealtimeWebSocketService) HasSession(sessionID string) bool {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	_, exists := s.sessions[sessionID]
	return exists
}

// GetAllSessions возвращает копию всех активных сессий
func (s *RealtimeWebSocketService) GetAllSessions() map[string]*Session {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()

	// Создаем копию map для безопасного возврата
	sessionsCopy := make(map[string]*Session, len(s.sessions))
	for id, session := range s.sessions {
		sessionsCopy[id] = session
	}
	return sessionsCopy
}

// GetSessionsCount возвращает количество активных сессий
func (s *RealtimeWebSocketService) GetSessionsCount() int {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	return len(s.sessions)
}

// StartSession инициализирует сессию для обработки аудио в реальном времени
func (s *RealtimeWebSocketService) StartSession(sessionID string, options *interfaces.ProcessorSessionOptions) error {
	session := s.createSession(sessionID, options.Callback)

	// Создаем параметры для сессии реалтайм транскрипции
	realtimeOptions := &interfaces.RealtimeSessionOptions{
		SessionID: sessionID,
		Format:    options.Format,
		Language:  DefaultLanguage,
	}

	// Запускаем сессию транскрипции
	_, resultCh, err := s.transcriberService.StartSession(context.Background(), realtimeOptions)
	if err != nil {
		return fmt.Errorf("failed to start realtime transcription session: %w", err)
	}

	// Устанавливаем канал результатов
	session.SetResultChannel(resultCh)

	// Запускаем горутину для обработки результатов транскрипции
	go s.handleRealtimeResults(session)

	return nil
}

// HandleAudioMessage обрабатывает сообщение с аудиоданными в реальном времени
func (s *RealtimeWebSocketService) HandleAudioMessage(sessionID string, data json.RawMessage) error {
	// Разбор аудиоданных
	var audioData AudioData
	if err := json.Unmarshal(data, &audioData); err != nil {
		s.logger.Error(fmt.Sprintf("Error parsing audio data: %v", err))
		return fmt.Errorf(ErrorMessageInvalidAudioData+": %w", err)
	}

	// Проверяем, не пусты ли данные
	if len(audioData.Samples) == 0 {
		s.logger.Error("Empty audio data received")
		return fmt.Errorf(ErrorMessageEmptyAudioData)
	}

	// Отправляем аудиоданные в реалтайм транскрипцию
	if err := s.transcriberService.AppendAudio(context.Background(), sessionID, audioData.Samples); err != nil {
		s.logger.Error(fmt.Sprintf("Error appending audio to realtime session: %v", err))
		return fmt.Errorf(ErrorMessageFailedToAppendAudio+": %w", err)
	}

	return nil
}

// HandleStopMessage обрабатывает сообщение об окончании записи в реальном времени
func (s *RealtimeWebSocketService) HandleStopMessage(sessionID string) (string, error) {
	session := s.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("RealtimeWebSocketService: session not found")
	}

	// Сохраняем последний известный текст
	result := session.GetLastTranscriptText()

	// Завершаем сессию транскрипции
	if finalResult, err := s.transcriberService.CompleteSession(context.Background(), session.GetID()); err == nil && finalResult != nil {
		// Обновляем результат, если он получен успешно
		result = finalResult.Text
	} else if err != nil {
		s.logger.Error(ErrorMessageFailedToCompleteSession + ": " + err.Error())
	}

	// Исправляем текст через LLM
	fixedResult, err := s.textGenerationService.FixText(context.Background(), result)
	if err != nil {
		s.logger.Error(ErrorMessageFailedToFixText + ": " + err.Error())
		// Продолжаем с исходным текстом, если исправление не удалось
		fixedResult = result
	}

	return fixedResult, nil
}

// CloseSession закрывает сессию и освобождает ресурсы
func (s *RealtimeWebSocketService) CloseSession(sessionID string) error {
	// Закрываем сессию реального времени
	if sessionID != "" {
		if err := s.transcriberService.CloseSession(context.Background(), sessionID); err != nil {
			s.logger.Warn(fmt.Sprintf("Error closing realtime session: %v", err))
		} else {
			s.logger.Info(fmt.Sprintf("Successfully closed realtime session: %s", sessionID))
		}
	}

	// Удаляем сессию из map'а
	s.RemoveSession(sessionID)

	return nil
}

// handleRealtimeResults обрабатывает результаты транскрипции в реальном времени
// и отправляет их клиенту через WebSocket
func (s *RealtimeWebSocketService) handleRealtimeResults(session *Session) {
	resultCh := session.GetResultChannel()
	if resultCh == nil {
		s.logger.Error(fmt.Sprintf("Realtime result channel is nil for session: %s", session.GetID()))
		return
	}

	// Обрабатываем результаты из канала, пока он не закроется
	for result := range resultCh {
		if result == nil {
			continue
		}

		// Сохраняем последний текст
		session.SetLastTranscriptText(result.Text)

		// Вызываем callback
		session.CallCallback(result.Text)
	}

	s.logger.Info(fmt.Sprintf("Realtime result channel closed for session: %s", session.GetID()))
}
