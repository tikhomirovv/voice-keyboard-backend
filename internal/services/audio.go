package services

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// Константы для WAV формата
const (
	// Стандартные параметры аудио для речи
	// WAV_SAMPLE_RATE     uint32 = 16000 // 16 кГц - обычно используется для речи
	WAV_CHANNELS        uint16 = 1 // моно
	WAV_BITS_PER_SAMPLE uint16 = 8 // 16 бит
)

// audioSession представляет внутреннюю структуру для хранения информации об аудиофайле
type audioSession struct {
	filePath   string        // Путь к файлу
	sampleRate uint32        // Частота дискретизации
	userID     uint64        // ID пользователя
	sessionID  string        // ID сессии
	file       *os.File      // Дескриптор файла (если файл открыт)
	logger     logger.Logger // Логгер для сообщений
}

// writeData записывает аудиоданные в файл
func (s *audioSession) writeData(data []byte) error {
	if s.file == nil {
		return fmt.Errorf("session file is closed or not initialized")
	}

	if _, err := s.file.Write(data); err != nil {
		return fmt.Errorf("error writing to audio file: %w", err)
	}

	return nil
}

// close закрывает файл и конвертирует его в WAV формат
func (s *audioSession) close() (string, error) {
	if s.file == nil {
		return s.filePath, nil // Файл уже закрыт
	}

	// Получаем путь к файлу до закрытия дескриптора
	tmpFilePath := s.filePath

	// Синхронизируем и закрываем файл
	if err := s.file.Sync(); err != nil {
		return "", fmt.Errorf("error syncing audio file: %w", err)
	}

	if err := s.file.Close(); err != nil {
		return "", fmt.Errorf("error closing audio file: %w", err)
	}

	// Обнуляем дескриптор файла после закрытия
	s.file = nil

	// Если путь уже имеет расширение .wav, просто возвращаем его
	if filepath.Ext(tmpFilePath) == ".wav" {
		return tmpFilePath, nil
	}

	// Конвертируем файл в WAV формат
	wavFilePath, err := s.convertToWavFile(tmpFilePath)
	if err != nil {
		return "", fmt.Errorf("error converting to WAV format: %w", err)
	}

	// Обновляем путь к файлу после конвертации
	s.filePath = wavFilePath
	s.logger.Debug(fmt.Sprintf("Audio file closed and converted to WAV: %s", wavFilePath))

	return wavFilePath, nil
}

// convertToWavFile преобразует файл с сырыми аудиоданными в WAV-формат
// Этот метод теперь является частью audioSession и имеет доступ к sampleRate
func (s *audioSession) convertToWavFile(rawFilePath string) (string, error) {
	// Проверяем существование исходного файла
	if _, err := os.Stat(rawFilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("raw audio file does not exist: %s", rawFilePath)
	}

	// Открываем исходный файл с сырыми данными
	rawFile, err := os.Open(rawFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open raw audio file: %w", err)
	}

	// Получаем информацию о размере файла
	fileInfo, err := rawFile.Stat()
	if err != nil {
		rawFile.Close() // Закрываем файл в случае ошибки
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	dataSize := fileInfo.Size()

	// Создаем путь для WAV-файла (заменяем расширение)
	wavFilePath := filepath.Join(filepath.Dir(rawFilePath),
		fmt.Sprintf("%s.wav", filepath.Base(rawFilePath[:len(rawFilePath)-4])))

	// Создаем WAV-файл
	wavFile, err := os.Create(wavFilePath)
	if err != nil {
		rawFile.Close() // Закрываем исходный файл в случае ошибки
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}

	// Записываем WAV-заголовок
	if err := s.writeWavHeader(wavFile, uint32(dataSize)); err != nil {
		rawFile.Close()
		wavFile.Close()
		return "", fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Копируем сырые аудиоданные из исходного файла в WAV-файл
	if _, err := io.Copy(wavFile, rawFile); err != nil {
		rawFile.Close()
		wavFile.Close()
		return "", fmt.Errorf("failed to copy audio data: %w", err)
	}

	// Синхронизируем данные на диск перед закрытием
	if err := wavFile.Sync(); err != nil {
		rawFile.Close()
		wavFile.Close()
		return "", fmt.Errorf("error syncing WAV file: %w", err)
	}

	// Закрываем файлы явно, чтобы убедиться, что все операции завершены,
	// перед удалением исходного файла
	rawFile.Close()
	wavFile.Close()

	// Удаляем временный файл после успешной конвертации
	if err := os.Remove(rawFilePath); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to remove temporary file %s: %v", rawFilePath, err))
		// Не прерываем работу, если не удалось удалить временный файл,
		// но логируем предупреждение
	} else {
		s.logger.Debug(fmt.Sprintf("Temporary file removed: %s", rawFilePath))
	}

	s.logger.Debug(fmt.Sprintf("Successfully converted %s to WAV format: %s", rawFilePath, wavFilePath))
	return wavFilePath, nil
}

// writeWavHeader записывает заголовок WAV-файла используя константные параметры формата
func (s *audioSession) writeWavHeader(file *os.File, dataSize uint32) error {
	// Вычисляем ByteRate и BlockAlign
	byteRate := s.sampleRate * uint32(WAV_CHANNELS) * uint32(WAV_BITS_PER_SAMPLE) / 8
	blockAlign := WAV_CHANNELS * WAV_BITS_PER_SAMPLE / 8

	// Вычисляем размер всего файла (размер данных + размер заголовка - 8)
	fileSize := dataSize + 36 // 44 - 8 = 36 (размер RIFF-заголовка без "RIFF" и размера файла)

	// Создаем буфер для заголовка
	var header [44]byte

	// RIFF заголовок
	copy(header[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:8], fileSize)
	copy(header[8:12], []byte("WAVE"))

	// fmt подсекция
	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16) // размер fmt секции (16 байт)
	binary.LittleEndian.PutUint16(header[20:22], 1)  // формат аудио (1 = PCM)
	binary.LittleEndian.PutUint16(header[22:24], WAV_CHANNELS)
	binary.LittleEndian.PutUint32(header[24:28], s.sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], blockAlign)
	binary.LittleEndian.PutUint16(header[34:36], WAV_BITS_PER_SAMPLE)

	// data подсекция
	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	// Записываем заголовок в файл
	if _, err := file.Write(header[:]); err != nil {
		return fmt.Errorf("error writing WAV header: %w", err)
	}

	return nil
}

// remove удаляет временный файл аудиосессии
func (s *audioSession) remove() error {
	if s.filePath == "" {
		return nil
	}

	// Закрываем файл, если он всё ещё открыт
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}

	// Проверяем существование файла
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	if err := os.Remove(s.filePath); err != nil {
		return fmt.Errorf("failed to delete audio file: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Audio file deleted: %s", s.filePath))
	s.filePath = ""
	return nil
}

// AudioService представляет сервис для работы с аудиоданными
type AudioService struct {
	logger        logger.Logger
	audioSessions map[string]*audioSession // Хранилище аудиосессий, ключ - sessionID
}

// NewAudioService создает новый экземпляр сервиса для работы с аудиоданными
func NewAudioService(logger logger.Logger) *AudioService {
	return &AudioService{
		logger:        logger,
		audioSessions: make(map[string]*audioSession),
	}
}

// GetAudioFilePath генерирует путь к файлу по ID сессии
// Может использоваться как внутренними методами, так и внешними компонентами
func (s *AudioService) GetAudioFilePath(userID uint64, sessionID string, isTemp bool) string {
	tempDir := s.GetTempDir(userID)
	extension := ".wav"
	if isTemp {
		extension = ".tmp"
	}
	return filepath.Join(tempDir, fmt.Sprintf("%s%s", sessionID, extension))
}

func (s *AudioService) GetTempDir(userID uint64) string {
	return filepath.Join(os.TempDir(), "voice-keyboard", fmt.Sprintf("%d", userID))
}

// Create создает файл для сохранения аудиоданных и регистрирует новую аудиосессию
func (s *AudioService) Create(userID uint64, sessionID string, sampleRate uint32) (string, error) {
	// Создаем временную директорию, если её нет
	tempDir := s.GetTempDir(userID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Генерируем путь к файлу
	audioFilePath := s.GetAudioFilePath(userID, sessionID, true)

	// Создаем файл
	audioFile, err := os.Create(audioFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create audio file: %w", err)
	}

	// Создаем и сохраняем сессию
	session := &audioSession{
		filePath:   audioFilePath,
		sampleRate: sampleRate,
		userID:     userID,
		sessionID:  sessionID,
		file:       audioFile,
		logger:     s.logger,
	}
	s.audioSessions[sessionID] = session

	s.logger.Debug(fmt.Sprintf("Created temporary audio file: %s", audioFilePath))
	return audioFilePath, nil
}

// WriteData записывает аудиоданные в файл для указанной сессии
func (s *AudioService) WriteData(sessionID string, data []byte) error {
	session, exists := s.audioSessions[sessionID]
	if !exists {
		return fmt.Errorf("audio session not found: %s", sessionID)
	}

	return session.writeData(data)
}

// Close закрывает файл с аудиоданными и конвертирует его в WAV-формат
// Возвращает путь к WAV-файлу, готовому для обработки
func (s *AudioService) Close(sessionID string) (string, error) {
	session, exists := s.audioSessions[sessionID]
	if !exists {
		return "", fmt.Errorf("audio session not found: %s", sessionID)
	}

	return session.close()
}

// Remove удаляет аудиофайлы и очищает данные сессии по её ID
func (s *AudioService) Remove(userID uint64, sessionID string) error {
	// Сначала всегда генерируем пути к возможным файлам
	tmpFilePath := s.GetAudioFilePath(userID, sessionID, true)  // путь к .tmp файлу
	wavFilePath := s.GetAudioFilePath(userID, sessionID, false) // путь к .wav файлу

	var deleteErrors []error

	// Пытаемся удалить .tmp файл, если он существует
	if _, err := os.Stat(tmpFilePath); !os.IsNotExist(err) {
		if err := os.Remove(tmpFilePath); err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to delete TMP file %s: %v", tmpFilePath, err))
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete TMP file: %w", err))
		} else {
			s.logger.Debug(fmt.Sprintf("TMP file deleted: %s", tmpFilePath))
		}
	}

	// Пытаемся удалить .wav файл, если он существует
	if _, err := os.Stat(wavFilePath); !os.IsNotExist(err) {
		if err := os.Remove(wavFilePath); err != nil {
			s.logger.Warn(fmt.Sprintf("Failed to delete WAV file %s: %v", wavFilePath, err))
			deleteErrors = append(deleteErrors, fmt.Errorf("failed to delete WAV file: %w", err))
		} else {
			s.logger.Debug(fmt.Sprintf("WAV file deleted: %s", wavFilePath))
		}
	}

	// Проверяем, есть ли сессия с указанным ID
	session, exists := s.audioSessions[sessionID]
	if exists {
		session.remove()
		delete(s.audioSessions, sessionID)
		s.logger.Debug(fmt.Sprintf("Audio session removed: %s", sessionID))
	}

	// Если были ошибки при удалении, возвращаем первую из них
	if len(deleteErrors) > 0 {
		return deleteErrors[0]
	}

	return nil
}
