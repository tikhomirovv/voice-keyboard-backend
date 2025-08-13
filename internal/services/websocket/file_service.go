// Пока не используется
// Нужно переделать по аналогии с realtime_service.go
// Этот вид во время рефакторинга и вайб-кодинга
// Сохранено для того чтобы видеть логику работы с аудио-сервисом

package websocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// FileSession представляет внутреннюю сессию для обработки аудио с сохранением в файл
type FileSession struct {
	id string
	// Поля для работы с файловой транскрипцией
	audioFilePath string   // Путь к временному файлу с аудиоданными
	audioFile     *os.File // Дескриптор файла для записи аудиоданных
	format        string   // Формат аудио (pcm16 и т.д.)
	sampleRate    uint32   // Частота дискретизации
	callback      func(text string)
	mutex         sync.RWMutex // Мьютекс для защиты конкурентного доступа
}

// GetID возвращает ID сессии (безопасно, так как id не изменяется)
func (s *FileSession) GetID() string {
	return s.id
}

// SetAudioFilePath безопасно устанавливает путь к аудиофайлу
func (s *FileSession) SetAudioFilePath(path string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.audioFilePath = path
}

// GetAudioFilePath безопасно возвращает путь к аудиофайлу
func (s *FileSession) GetAudioFilePath() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.audioFilePath
}

// SetAudioFile безопасно устанавливает дескриптор файла
func (s *FileSession) SetAudioFile(file *os.File) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.audioFile = file
}

// GetAudioFile безопасно возвращает дескриптор файла
func (s *FileSession) GetAudioFile() *os.File {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.audioFile
}

// SetFormat безопасно устанавливает формат аудио
func (s *FileSession) SetFormat(format string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.format = format
}

// GetFormat безопасно возвращает формат аудио
func (s *FileSession) GetFormat() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.format
}

// SetSampleRate безопасно устанавливает частоту дискретизации
func (s *FileSession) SetSampleRate(sampleRate uint32) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sampleRate = sampleRate
}

// GetSampleRate безопасно возвращает частоту дискретизации
func (s *FileSession) GetSampleRate() uint32 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.sampleRate
}

// SetCallback безопасно устанавливает callback функцию
func (s *FileSession) SetCallback(callback func(text string)) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.callback = callback
}

// GetCallback безопасно возвращает callback функцию
func (s *FileSession) GetCallback() func(text string) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.callback
}

// CallCallback безопасно вызывает callback функцию с проверкой на nil
func (s *FileSession) CallCallback(text string) {
	s.mutex.RLock()
	callback := s.callback
	s.mutex.RUnlock()

	if callback != nil {
		callback(text)
	}
}

// IsActive проверяет, активна ли сессия (есть ли открытый файл)
func (s *FileSession) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.audioFile != nil
}

// FileWebSocketService реализует обработку WebSocket сообщений с сохранением в файл
type FileWebSocketService struct {
	logger                logger.Logger
	audioService          interfaces.AudioServiceInterface
	textGenerationService interfaces.LLMTextGenerationServiceInterface
	sessions              map[string]*FileSession
	sessionsMutex         sync.RWMutex // Мьютекс для защиты map sessions
}

// NewFileWebSocketService создает новый сервис для обработки WebSocket с сохранением в файл
func NewFileWebSocketService(
	logger logger.Logger,
	audioService interfaces.AudioServiceInterface,
	textGenerationService interfaces.LLMTextGenerationServiceInterface,
) *FileWebSocketService {
	return &FileWebSocketService{
		logger:                logger,
		audioService:          audioService,
		textGenerationService: textGenerationService,
		sessions:              make(map[string]*FileSession),
	}
}

func (s *FileWebSocketService) createSession(sessionID string, options *interfaces.ProcessorSessionOptions) *FileSession {
	session := &FileSession{
		id:         sessionID,
		format:     options.Format,
		sampleRate: options.SampleRate,
		callback:   options.Callback,
	}

	s.AddSession(session)
	return session
}

// AddSession безопасно добавляет сессию в map
func (s *FileWebSocketService) AddSession(session *FileSession) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	s.sessions[session.id] = session
}

// GetSession безопасно возвращает сессию по ID
func (s *FileWebSocketService) GetSession(sessionID string) *FileSession {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	return s.sessions[sessionID]
}

// RemoveSession безопасно удаляет сессию из map
func (s *FileWebSocketService) RemoveSession(sessionID string) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	delete(s.sessions, sessionID)
}

// HasSession проверяет существование сессии
func (s *FileWebSocketService) HasSession(sessionID string) bool {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	_, exists := s.sessions[sessionID]
	return exists
}

// GetAllSessions возвращает копию всех активных сессий
func (s *FileWebSocketService) GetAllSessions() map[string]*FileSession {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()

	// Создаем копию map для безопасного возврата
	sessionsCopy := make(map[string]*FileSession, len(s.sessions))
	for id, session := range s.sessions {
		sessionsCopy[id] = session
	}
	return sessionsCopy
}

// GetSessionsCount возвращает количество активных сессий
func (s *FileWebSocketService) GetSessionsCount() int {
	s.sessionsMutex.RLock()
	defer s.sessionsMutex.RUnlock()
	return len(s.sessions)
}

// StartSession инициализирует сессию для обработки аудио с сохранением в файл
func (s *FileWebSocketService) StartSession(sessionID string, options *interfaces.ProcessorSessionOptions) error {
	session := s.createSession(sessionID, options)

	// Создаем файл для записи аудиоданных
	// Используем фиктивный userID = 0, так как в новой архитектуре userID не передается
	audioFilePath, err := s.audioService.Create(options.UserID, sessionID, session.GetSampleRate())
	if err != nil {
		return fmt.Errorf(ErrorMessageFailedToCreateFile+": %w", err)
	}

	session.SetAudioFilePath(audioFilePath)

	s.logger.Info(fmt.Sprintf("Started file-based session: %s; file path: %s", sessionID, audioFilePath))

	return nil
}

// HandleAudioMessage обрабатывает сообщение с аудиоданными с сохранением в файл
func (s *FileWebSocketService) HandleAudioMessage(sessionID string, data json.RawMessage) error {
	session := s.GetSession(sessionID)
	if session == nil {
		return fmt.Errorf("FileWebSocketService: session not found")
	}

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

	// Декодируем данные из Base64
	rawAudioBytes, err := base64.StdEncoding.DecodeString(audioData.Samples)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error decoding base64 audio data: %v", err))
		return fmt.Errorf(ErrorMessageFailedToDecodeAudio+": %w", err)
	}

	// Создаем структуру с форматом аудио
	format := &AudioFormat{
		Format:     session.GetFormat(),
		SampleRate: session.GetSampleRate(),
	}

	// Получаем процессор для указанного формата
	processor, err := ValidateAndGetProcessor(format)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error with audio format: %v", err))
		return fmt.Errorf(ErrorMessageAudioFormatError+": %w", err)
	}

	// Обработка аудиоданных с соответствующим процессором
	processedData, err := processor.Process(rawAudioBytes)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio: %v", err))
		return fmt.Errorf(ErrorMessageFailedToProcessAudio+": %w", err)
	}

	// Записываем данные в файл
	if err := s.audioService.WriteData(sessionID, processedData); err != nil {
		s.logger.Error(fmt.Sprintf("Error writing to audio file: %v", err))
		return fmt.Errorf(ErrorMessageFailedToWriteFile+": %w", err)
	}

	return nil
}

// HandleStopMessage обрабатывает сообщение об окончании записи с сохранением в файл
func (s *FileWebSocketService) HandleStopMessage(sessionID string) (string, error) {
	session := s.GetSession(sessionID)
	if session == nil {
		return "", fmt.Errorf("FileWebSocketService: session not found")
	}

	// Закрываем аудиофайл
	if _, err := s.audioService.Close(sessionID); err != nil {
		s.logger.Error(fmt.Sprintf("Error closing audio file: %v", err))
		return "", fmt.Errorf(ErrorMessageFailedToCloseFile+": %w", err)
	}

	// Обрабатываем собранные аудиоданные из файла
	result, err := s.processAudioDataFromFile(sessionID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio data: %v", err))
		return "", fmt.Errorf(ErrorMessageFailedToProcessAudio+": %w", err)
	}

	// Исправляем текст через LLM
	fixedResult, err := s.textGenerationService.FixText(context.Background(), result)
	if err != nil {
		s.logger.Error(fmt.Sprintf(ErrorMessageFailedToFixText+": %v", err))
		// Продолжаем с исходным текстом, если исправление не удалось
		fixedResult = result
	}

	return fixedResult, nil
}

// CloseSession закрывает сессию и освобождает ресурсы
func (s *FileWebSocketService) CloseSession(sessionID string) error {
	// Закрываем аудиофайл
	if _, err := s.audioService.Close(sessionID); err != nil {
		s.logger.Warn(fmt.Sprintf(ErrorMessageFailedToCloseFile+": %v", err))
	}

	// Удаляем сессию из map'а
	s.RemoveSession(sessionID)

	return nil
}

// processAudioDataFromFile обрабатывает собранные аудиоданные из файла и возвращает результат распознавания
func (s *FileWebSocketService) processAudioDataFromFile(sessionID string) (string, error) {
	// В реальной реализации здесь должна быть обработка файла через транскриптор
	// Пока возвращаем заглушку
	return MockTranscriptionResult, nil
}
