package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

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
		ID:         sessionID,
		Format:     format,
		Language:   language,
		ResultCh:   resultCh,
		Created:    time.Now(),
		LastActive: time.Now(),
		config:     config,
		logger:     logger,
		closed:     false,
		closeCh:    make(chan struct{}),
		ready:      false,
		// Флаг активности речи и мьютекс для его защиты
		isSpeech:    true, // Начинаем с true - речь активна
		speechMutex: sync.RWMutex{},

		// Флаг ожидания коммита и канал для события committed
		waitingCommit: false,               // Начинаем с false - не ждем коммит
		commitCh:      make(chan struct{}), // Небуферизированный канал

		// Карта элементов разговора
		conversationItems: make(map[string]*ConversationItem),
		itemsMutex:        sync.RWMutex{},
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
	s.logger.Debug(fmt.Sprintf("Session %s: Connecting to OpenAI Realtime API", s.ID))
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
	s.logger.Info(fmt.Sprintf("Session %s: Connected to OpenAI Realtime API", s.ID))
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
				Type: "server_vad",
				// Type:      "semantic_vad",
				// Eagerness: "low",
				// Threshold:         0.5,
				PrefixPaddingMS:   200,
				SilenceDurationMS: 200,
			},
			// TurnDetection: nil,
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
		s.logger.Debug(fmt.Sprintf("Session %s: Exiting OpenAI Realtime API message handler", s.ID))
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
					s.logger.Debug(fmt.Sprintf("Session %s: Connection closed normally", s.ID))
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
			case eventTypeInputAudioBufferSpeechStopped:
				s.handleAudioBufferSpeechStopped(&event)
			case eventTypeError:
				s.handleError(&event)
			case eventTypeTranscriptionSessionCreated:
				// Добавляем обработку события создания сессии транскрипции
				s.logger.Debug(fmt.Sprintf("Session %s: Transcription session created", s.ID))
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
	// s.logger.Debug(fmt.Sprintf("Session %s: Delta transcript: %s", s.ID, event.Delta))
}

func (s *RealtimeSession) handleAudioBufferSpeechStarted(_ *RealtimeEvent) {
	s.logger.Debug(fmt.Sprintf("Session %s: Audio buffer speech started", s.ID))
}

func (s *RealtimeSession) handleAudioBufferSpeechStopped(_ *RealtimeEvent) {
	s.logger.Debug(fmt.Sprintf("Session %s: Audio buffer speech stopped", s.ID))
}

// handleTranscriptionCompleted обрабатывает завершенные результаты транскрипции
func (s *RealtimeSession) handleTranscriptionCompleted(event *RealtimeEvent) {
	// Обновляем элемент в карте разговора
	s.UpdateConversationItem(event.ItemID, ItemCompleted, event.Transcript)

	// Отправляем результат в канал
	result := &dto.TranscriberResult{
		Text: event.Transcript,
	}

	select {
	case s.ResultCh <- result:
		s.logger.Debug(fmt.Sprintf("Session %s: Sent transcript: %s", s.ID, event.Transcript))
	default:
		s.logger.Warn(fmt.Sprintf("Session %s: Failed to send transcript: channel full or closed", s.ID))
	}
}

// handleAudioBufferCommitted обрабатывает события завершения буфера аудио
func (s *RealtimeSession) handleAudioBufferCommitted(event *RealtimeEvent) {
	s.logger.Debug(fmt.Sprintf("Session %s: Audio buffer committed: item_id=%s, previous_item_id=%s",
		s.ID, event.ItemID, event.PreviousItemID))

	// Добавляем элемент в карту разговора
	s.AddConversationItem(event.ItemID, event.PreviousItemID, ItemCommitted)

	// Отправляем сигнал о получении committed события, если ждем коммит
	if s.IsWaitingCommit() {
		select {
		case s.commitCh <- struct{}{}:
			s.logger.Debug(fmt.Sprintf("Session %s: Sent committed signal", s.ID))
		default:
			// Неблокирующая отправка - если никто не ждет, просто игнорируем
		}
	}
}

// handleError обрабатывает события ошибок от Realtime API
func (s *RealtimeSession) handleError(event *RealtimeEvent) {
	if event.Error != nil {
		s.logger.Error(fmt.Sprintf("Session %s: Received error from Realtime API: %+v", s.ID, event.Error))
		// При ошибке коммита пустого буфера отправляем сигнал, если ждем коммит
		if event.Error.Code == "input_audio_buffer_commit_empty" && s.IsWaitingCommit() {
			select {
			case s.commitCh <- struct{}{}:
				s.logger.Debug(fmt.Sprintf("Session %s: Sent commit error signal for empty buffer", s.ID))
			default:
				// Неблокирующая отправка - если никто не ждет, просто игнорируем
			}
		}
	}
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

	s.logger.Debug(fmt.Sprintf("Session %s: Sent audio buffer commit request", s.ID))
	return nil
}

// sendJSON отправляет JSON объект в WebSocket соединение
func (s *RealtimeSession) sendJSON(v any) error {
	if s.closed || s.conn == nil {
		return fmt.Errorf("session is closed or not connected")
	}

	return s.conn.WriteJSON(v)
}

// IsSpeech возвращает состояние активности речи с блокировкой
func (s *RealtimeSession) IsSpeech() bool {
	s.speechMutex.RLock()
	defer s.speechMutex.RUnlock()
	return s.isSpeech
}

// SetSpeech безопасно устанавливает состояние активности речи
func (s *RealtimeSession) SetSpeech(speech bool) {
	s.speechMutex.Lock()
	defer s.speechMutex.Unlock()
	s.isSpeech = speech
}

// IsWaitingCommit возвращает состояние ожидания коммита с блокировкой
func (s *RealtimeSession) IsWaitingCommit() bool {
	s.speechMutex.RLock()
	defer s.speechMutex.RUnlock()
	return s.waitingCommit
}

// SetWaitingCommit безопасно устанавливает состояние ожидания коммита
func (s *RealtimeSession) SetWaitingCommit(waiting bool) {
	s.speechMutex.Lock()
	defer s.speechMutex.Unlock()
	s.waitingCommit = waiting
}

// AddConversationItem добавляет новый элемент разговора
func (s *RealtimeSession) AddConversationItem(itemID string, previousItemID string, status ItemStatus) {
	s.itemsMutex.Lock()
	defer s.itemsMutex.Unlock()

	s.conversationItems[itemID] = &ConversationItem{
		ItemID:         itemID,
		PreviousItemID: previousItemID,
		Status:         status,
	}
}

// UpdateConversationItem обновляет статус элемента разговора
func (s *RealtimeSession) UpdateConversationItem(itemID string, status ItemStatus, transcript string) {
	s.itemsMutex.Lock()
	defer s.itemsMutex.Unlock()

	if item, exists := s.conversationItems[itemID]; exists {
		item.Status = status
		if transcript != "" {
			item.Transcript = transcript
		}
	}
}

// GetCompletedTranscripts возвращает все завершенные транскрипции в правильном порядке
func (s *RealtimeSession) GetCompletedTranscripts() string {
	s.itemsMutex.RLock()
	defer s.itemsMutex.RUnlock()

	// Собираем все завершенные элементы
	var items []*ConversationItem
	for _, item := range s.conversationItems {
		if item.Status == ItemCompleted {
			items = append(items, item)
		}
	}

	// Сортируем по цепочке previous_item_id
	sortedItems := s.sortByPreviousItemID(items)

	// Конкатенируем транскрипции
	var result strings.Builder
	for _, item := range sortedItems {
		if item.Transcript != "" {
			if result.Len() > 0 {
				result.WriteString(" ")
			}
			result.WriteString(item.Transcript)
		}
	}

	return result.String()
}

// sortByPreviousItemID сортирует элементы по цепочке previous_item_id
func (s *RealtimeSession) sortByPreviousItemID(items []*ConversationItem) []*ConversationItem {
	if len(items) == 0 {
		return items
	}

	// Создаем карту для быстрого поиска элементов по ID
	itemMap := make(map[string]*ConversationItem)
	for _, item := range items {
		itemMap[item.ItemID] = item
	}

	// Строим цепочку элементов, начиная с первого (без previous_item_id)
	var sorted []*ConversationItem
	visited := make(map[string]bool)

	// Находим первый элемент (тот, у которого previous_item_id пустой или не найден)
	var current *ConversationItem
	for _, item := range items {
		if item.PreviousItemID == "" || itemMap[item.PreviousItemID] == nil {
			current = item
			break
		}
	}

	// Если не нашли первый элемент, возвращаем как есть
	if current == nil {
		return items
	}

	// Строим цепочку, избегая циклов
	for current != nil && !visited[current.ItemID] {
		visited[current.ItemID] = true
		sorted = append(sorted, current)

		// Ищем следующий элемент, у которого previous_item_id равен current.ItemID
		var next *ConversationItem
		for _, item := range items {
			if item.PreviousItemID == current.ItemID {
				next = item
				break
			}
		}
		current = next
	}

	return sorted
}

// GetPendingItemIDs возвращает ID элементов, которые еще не завершены
func (s *RealtimeSession) GetPendingItemIDs() []string {
	s.itemsMutex.RLock()
	defer s.itemsMutex.RUnlock()

	var pending []string
	for itemID, item := range s.conversationItems {
		if item.Status == ItemCommitted {
			pending = append(pending, itemID)
		}
	}

	return pending
}

// WaitForCompletion ожидает завершения всех ожидающих элементов с периодической проверкой
func (s *RealtimeSession) WaitForCompletion(ctx context.Context) bool {
	// Таймер для периодической проверки
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Таймаут для общего ожидания
	timeout := time.After(transcriptionWaitTimeout)

	for {
		select {
		case <-ticker.C:
			// Проверяем статус всех элементов
			pendingItems := s.GetPendingItemIDs()
			if len(pendingItems) == 0 {
				s.logger.Info(fmt.Sprintf("Session %s: All items completed", s.ID))
				return true
			}
			s.logger.Debug(fmt.Sprintf("Session %s: Still waiting for %d items: %v", s.ID, len(pendingItems), pendingItems))
		case <-timeout:
			s.logger.Warn(fmt.Sprintf("Session %s: Timeout waiting for completion of items", s.ID))
			return false
		case <-ctx.Done():
			s.logger.Debug(fmt.Sprintf("Session %s: Context done while waiting for completion", s.ID))
			return false
		case <-s.closeCh:
			s.logger.Debug(fmt.Sprintf("Session %s: Connection closed while waiting for completion", s.ID))
			return false
		}
	}
}

// WaitForCommitResult ожидает события committed после коммита буфера
func (s *RealtimeSession) WaitForCommitResult(ctx context.Context) (bool, error) {
	// Устанавливаем флаг ожидания коммита
	s.SetWaitingCommit(true)
	defer s.SetWaitingCommit(false) // Сбрасываем флаг при выходе

	// Ждем события committed
	select {
	case <-s.commitCh:
		s.logger.Debug(fmt.Sprintf("Session %s: Received committed event", s.ID))
		return true, nil
	case <-time.After(commitTimeout):
		s.logger.Warn(fmt.Sprintf("Session %s: Timeout waiting for committed event", s.ID))
		return false, fmt.Errorf("timeout waiting for committed event")
	case <-ctx.Done():
		s.logger.Debug(fmt.Sprintf("Session %s: Context done while waiting for committed", s.ID))
		return false, ctx.Err()
	case <-s.closeCh:
		s.logger.Debug(fmt.Sprintf("Session %s: Connection closed while waiting for committed", s.ID))
		return false, fmt.Errorf("connection closed")
	}
}

// CommitAndWaitResult коммитит буфер и ждет результат коммита
func (s *RealtimeSession) CommitAndWaitResult(ctx context.Context) (bool, error) {
	// Коммитим буфер
	s.logger.Debug(fmt.Sprintf("Session %s: Committing audio buffer", s.ID))
	if err := s.commitAudioBuffer(); err != nil {
		s.logger.Warn(fmt.Sprintf("Session %s: Failed to commit audio buffer: %v", s.ID, err))
		return false, fmt.Errorf("failed to commit audio buffer: %w", err)
	}

	// Ждем события committed
	commitSuccessful, err := s.WaitForCommitResult(ctx)
	if err != nil {
		return false, err
	}

	if !commitSuccessful {
		// Коммит не удался (буфер пустой)
		s.logger.Debug(fmt.Sprintf("Session %s: Commit failed - buffer is empty", s.ID))
		return false, nil
	}

	// Ждем завершения всех элементов
	completed := s.WaitForCompletion(ctx)
	return completed, nil
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

	s.logger.Info(fmt.Sprintf("Session %s: Started", session.ID))

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

	// Устанавливаем флаг неактивности речи - начинаем ожидать результат
	session.SetSpeech(false)

	// Коммитим буфер и ждем результат
	s.logger.Info(fmt.Sprintf("Session %s: Processing STOP request", sessionID))
	completed, err := session.CommitAndWaitResult(ctx)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("Session %s: Error during commit and wait: %v", sessionID, err))
		// В случае ошибки ожидания коммита, все равно возвращаем результат
	} else if completed {
		// Успешно завершили все элементы
		s.logger.Info(fmt.Sprintf("Session %s: Successfully completed all items", sessionID))
	} else {
		// Коммит не удался (буфер пустой) или не все элементы завершились
		s.logger.Debug(fmt.Sprintf("Session %s: Commit failed or not all items completed", sessionID))
	}

	// Создаем результат на основе всех завершенных транскрипций
	finalText := session.GetCompletedTranscripts()
	finalResult := &dto.TranscriberResult{
		Text: finalText,
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

	s.logger.Info(fmt.Sprintf("Session %s: Closed", sessionID))

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
