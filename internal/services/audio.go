package services

import (
	"encoding/binary"
	"fmt"
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

// AudioService представляет сервис для работы с аудиоданными
type AudioService struct {
	logger logger.Logger
}

// NewAudioService создает новый экземпляр сервиса для работы с аудиоданными
func NewAudioService(logger logger.Logger) *AudioService {
	return &AudioService{
		logger: logger,
	}
}

// CreateAudioFile создает файл для сохранения аудиоданных
func (s *AudioService) CreateAudioFile(userID, sessionID string) (string, *os.File, error) {
	// Создаем временную директорию, если её нет
	tempDir := filepath.Join(os.TempDir(), "voice-keyboard")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Создаем временный файл для хранения аудиоданных (теперь с расширением .tmp)
	audioFilePath := filepath.Join(tempDir, fmt.Sprintf("%s_%s.tmp", userID, sessionID))
	audioFile, err := os.Create(audioFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create audio file: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Created temporary audio file: %s", audioFilePath))
	return audioFilePath, audioFile, nil
}

// WriteAudioData записывает аудиоданные в файл
func (s *AudioService) WriteAudioData(file *os.File, data []byte) error {
	if file == nil {
		return fmt.Errorf("invalid file descriptor")
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("error writing to audio file: %w", err)
	}

	return nil
}

// CloseAudioFile закрывает файл с аудиоданными и конвертирует его в WAV-формат
// Возвращает путь к WAV-файлу, готовому для обработки
func (s *AudioService) CloseAudioFile(file *os.File, sampleRate uint32) (string, error) {
	if file == nil {
		return "", nil
	}

	// Получаем путь к файлу до закрытия дескриптора
	tmpFilePath := file.Name()

	// Синхронизируем и закрываем файл
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("error syncing audio file: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("error closing audio file: %w", err)
	}

	// Конвертируем файл в WAV формат
	wavFilePath, err := s.convertToWavFile(tmpFilePath, sampleRate)
	if err != nil {
		return "", fmt.Errorf("error converting to WAV format: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Audio file closed and converted to WAV: %s", wavFilePath))
	return wavFilePath, nil
}

// convertToWavFile преобразует файл с сырыми аудиоданными в WAV-формат
// Возвращает путь к созданному WAV-файлу и удаляет исходный временный файл
// Приватный метод, используется только внутри AudioService
func (s *AudioService) convertToWavFile(rawFilePath string, sampleRate uint32) (string, error) {
	// Проверяем существование исходного файла
	if !s.FileExists(rawFilePath) {
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

	// Читаем все данные из файла
	data := make([]byte, dataSize)
	_, err = rawFile.Read(data)
	if err != nil {
		rawFile.Close()
		return "", fmt.Errorf("failed to read raw data: %w", err)
	}

	// Закрываем исходный файл после чтения всех данных
	rawFile.Close()

	// Создаем путь для WAV-файла (заменяем расширение)
	wavFilePath := filepath.Join(filepath.Dir(rawFilePath),
		fmt.Sprintf("%s.wav", filepath.Base(rawFilePath[:len(rawFilePath)-4])))

	// Создаем WAV-файл
	wavFile, err := os.Create(wavFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create WAV file: %w", err)
	}

	// Записываем WAV-заголовок
	if err := s.writeWavHeader(wavFile, uint32(dataSize), sampleRate); err != nil {
		wavFile.Close()
		return "", fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Записываем аудиоданные в WAV-файл
	if _, err := wavFile.Write(data); err != nil {
		wavFile.Close()
		return "", fmt.Errorf("failed to write audio data: %w", err)
	}

	// Синхронизируем данные на диск перед закрытием
	if err := wavFile.Sync(); err != nil {
		wavFile.Close()
		return "", fmt.Errorf("error syncing WAV file: %w", err)
	}

	// Закрываем файлы явно, чтобы убедиться, что все операции завершены,
	// перед удалением исходного файла
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

// RemoveAudioFile удаляет временный файл с аудиоданными
func (s *AudioService) RemoveAudioFile(filePath string) error {
	if filePath == "" {
		return nil
	}

	if !s.FileExists(filePath) {
		return nil
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete temporary audio file: %w", err)
	}

	s.logger.Debug(fmt.Sprintf("Temporary audio file deleted: %s", filePath))
	return nil
}

// FileExists проверяет существование файла
func (s *AudioService) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// writeWavHeader записывает заголовок WAV-файла используя константные параметры формата
func (s *AudioService) writeWavHeader(file *os.File, dataSize uint32, sampleRate uint32) error {
	// Вычисляем ByteRate и BlockAlign
	byteRate := sampleRate * uint32(WAV_CHANNELS) * uint32(WAV_BITS_PER_SAMPLE) / 8
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
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
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

// // ProcessAudioData обрабатывает аудиоданные из файла и возвращает результат распознавания
// func (s *AudioService) ProcessAudioData(sessionID, userID, filePath string, duration time.Duration) (AudioResult, error) {
// 	// Имитация задержки при обработке аудио
// 	time.Sleep(300 * time.Millisecond)

// 	// Получаем размер файла для информации
// 	fileInfo, err := os.Stat(filePath)
// 	if err != nil {
// 		return AudioResult{}, fmt.Errorf("error getting audio file info: %w", err)
// 	}

// 	// Логируем информацию об обработке
// 	s.logger.Info(fmt.Sprintf("Processing audio file %s (%d bytes) for session %s",
// 		filePath, fileInfo.Size(), sessionID))

// 	// FIXME: В реальной реализации здесь должен быть код для:
// 	// 1. Отправки аудиофайла в сторонний API для распознавания речи
// 	// 2. Ожидания и получения ответа от API
// 	// 3. Преобразования ответа в формат AudioResult
// 	// 4. Обработки возможных ошибок от API

// 	// Заглушка: возвращаем фиксированный текст
// 	return AudioResult{
// 		Text:     "Пример текста распознавания (из файла)",
// 		Language: "ru",
// 		Duration: float64(duration.Seconds()),
// 	}, nil
// }
