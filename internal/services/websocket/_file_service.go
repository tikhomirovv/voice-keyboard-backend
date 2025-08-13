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

	"gitlab.com/voice-keyboard/backend-go/internal/interfaces"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// FileWebSocketService реализует обработку WebSocket сообщений с сохранением в файл
type FileWebSocketService struct {
	logger                logger.Logger
	audioService          interfaces.AudioServiceInterface
	textGenerationService interfaces.LLMTextGenerationServiceInterface
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
	}
}

// StartSession инициализирует сессию для обработки аудио с сохранением в файл
func (s *FileWebSocketService) StartSession(session *WSSession) error {
	// Создаем файл для записи аудиоданных
	audioFilePath, err := s.audioService.Create(session.UserID, session.ID, session.SampleRate)
	if err != nil {
		return fmt.Errorf(ErrorMessageFailedToCreateFile+": %w", err)
	}

	session.AudioFilePath = audioFilePath

	s.logger.Info(fmt.Sprintf("Started file-based session: %s for user: %d", session.ID, session.UserID))

	return nil
}

// HandleAudioMessage обрабатывает сообщение с аудиоданными с сохранением в файл
func (s *FileWebSocketService) HandleAudioMessage(session *WSSession, message WebSocketMessage) error {
	// Разбор аудиоданных
	var audioData AudioData
	if err := json.Unmarshal(message.Data, &audioData); err != nil {
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
		Format:     session.Format,
		SampleRate: session.SampleRate,
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
	if err := s.audioService.WriteData(session.ID, processedData); err != nil {
		s.logger.Error(fmt.Sprintf("Error writing to audio file: %v", err))
		return fmt.Errorf(ErrorMessageFailedToWriteFile+": %w", err)
	}

	return nil
}

// HandleStopMessage обрабатывает сообщение об окончании записи с сохранением в файл
func (s *FileWebSocketService) HandleStopMessage(session *WSSession, message WebSocketMessage) error {
	// Закрываем аудиофайл
	if _, err := s.audioService.Close(session.ID); err != nil {
		s.logger.Error(fmt.Sprintf("Error closing audio file: %v", err))
		return fmt.Errorf(ErrorMessageFailedToCloseFile+": %w", err)
	}

	// Обрабатываем собранные аудиоданные из файла
	result, err := s.processAudioDataFromFile(session.ID, session.UserID)
	if err != nil {
		s.logger.Error(fmt.Sprintf("Error processing audio data: %v", err))
		return fmt.Errorf(ErrorMessageFailedToProcessAudio+": %w", err)
	}

	// Исправляем текст через LLM
	fixedResult, err := s.textGenerationService.FixText(context.Background(), result)
	if err != nil {
		s.logger.Error(fmt.Sprintf(ErrorMessageFailedToFixText+": %v", err))
		// Продолжаем с исходным текстом, если исправление не удалось
		fixedResult = result
	}

	// Отправляем результат обработки клиенту
	completedData := CompletedData{
		Text: fixedResult,
	}
	completedDataJSON, _ := json.Marshal(completedData)
	response := WebSocketMessage{
		Type:      MessageTypeCompleted,
		SessionID: session.ID,
		Data:      completedDataJSON,
	}

	// Отправляем сообщение клиенту
	if err := s.sendMessage(session, response); err != nil {
		s.logger.Error(fmt.Sprintf("Error sending result: %v", err))
		return fmt.Errorf(ErrorMessageFailedToSendResult+": %w", err)
	}

	return nil
}

// CloseSession закрывает сессию и освобождает ресурсы
func (s *FileWebSocketService) CloseSession(session *WSSession) error {
	// Закрываем аудиофайл
	if _, err := s.audioService.Close(session.ID); err != nil {
		s.logger.Warn(fmt.Sprintf(ErrorMessageFailedToCloseFile+": %v", err))
	}

	return nil
}

// processAudioDataFromFile обрабатывает собранные аудиоданные из файла и возвращает результат распознавания
func (s *FileWebSocketService) processAudioDataFromFile(sessionID string, userID uint64) (string, error) {
	// В реальной реализации здесь должна быть обработка файла через транскриптор
	// Пока возвращаем заглушку
	return MockTranscriptionResult, nil
}

// sendMessage отправляет сообщение клиенту
func (s *FileWebSocketService) sendMessage(session *WSSession, message WebSocketMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	session.Mutex.Lock()
	defer session.Mutex.Unlock()

	return session.Conn.WriteMessage(WebSocketTextMessage, data) // WebSocketTextMessage = websocket.TextMessage
}
