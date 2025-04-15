package websocket

import (
	"encoding/binary"
	"fmt"
)

// AudioProcessor представляет интерфейс для обработки аудиоданных разных форматов
type AudioProcessor interface {
	// Process обрабатывает сырые аудиоданные и возвращает обработанные данные
	Process(data []byte) ([]byte, error)
}

// AudioFormat содержит информацию о формате аудиоданных
type AudioFormat struct {
	// Format определяет тип данных: "i8", "i16", "f32" и т.д.
	Format string

	// SampleRate частота дискретизации (Гц)
	SampleRate uint32
}

// Int8Processor обрабатывает 8-битные целочисленные аудиоданные
type Int8Processor struct{}

// Process преобразует 8-битные беззнаковые значения (0-255) в знаковые (-128-127)
func (p *Int8Processor) Process(data []byte) ([]byte, error) {
	result := make([]byte, len(data))
	for i, b := range data {
		// Центрирование вокруг нуля: 0 -> -128, 128 -> 0, 255 -> 127
		result[i] = byte(int8(b - 128))
	}
	return result, nil
}

// GetBitsPerSample возвращает 8 для 8-битных данных
func (p *Int8Processor) GetBitsPerSample() uint16 {
	return 8
}

// GetFormatID возвращает 1 (PCM) для целочисленных данных
func (p *Int8Processor) GetFormatID() uint16 {
	return 1 // PCM формат
}

// Int16Processor обрабатывает 16-битные целочисленные аудиоданные
type Int16Processor struct{}

// Process преобразует 16-битные данные для правильного представления в WAV
func (p *Int16Processor) Process(data []byte) ([]byte, error) {
	// Проверяем, что количество байт кратно 2 (для 16-битных значений)
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("odd number of bytes for 16-bit samples: %d", len(data))
	}

	// Для 16-бит PCM данные уже должны быть центрированы вокруг нуля
	// Просто проверяем и корректно интерпретируем как int16
	result := make([]byte, len(data))
	for i := 0; i < len(data); i += 2 {
		// Читаем 16-битное значение в little-endian формате
		sample := int16(binary.LittleEndian.Uint16(data[i : i+2]))

		// Записываем значение обратно в байты (little-endian)
		binary.LittleEndian.PutUint16(result[i:i+2], uint16(sample))
	}

	return result, nil
}

// GetBitsPerSample возвращает 16 для 16-битных данных
func (p *Int16Processor) GetBitsPerSample() uint16 {
	return 16
}

// GetFormatID возвращает 1 (PCM) для целочисленных данных
func (p *Int16Processor) GetFormatID() uint16 {
	return 1 // PCM формат
}

// Int24Processor обрабатывает 24-битные целочисленные аудиоданные
type Int24Processor struct{}

// Process обрабатывает 24-битные PCM данные
func (p *Int24Processor) Process(data []byte) ([]byte, error) {
	// Проверяем, что количество байт кратно 3 (для 24-битных значений)
	if len(data)%3 != 0 {
		return nil, fmt.Errorf("data size not multiple of 3 for 24-bit samples: %d", len(data))
	}

	// Для 24-бит PCM нам нужно убедиться, что данные правильно упакованы
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// GetBitsPerSample возвращает 24 для 24-битных данных
func (p *Int24Processor) GetBitsPerSample() uint16 {
	return 24
}

// GetFormatID возвращает 1 (PCM) для целочисленных данных
func (p *Int24Processor) GetFormatID() uint16 {
	return 1 // PCM формат
}

// Float32Processor обрабатывает 32-битные данные с плавающей точкой
type Float32Processor struct{}

// Process преобразует 32-битные float данные для правильного представления в WAV
func (p *Float32Processor) Process(data []byte) ([]byte, error) {
	// Проверяем, что количество байт кратно 4 (для 32-битных float значений)
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("data size not multiple of 4 for 32-bit float samples: %d", len(data))
	}

	// Для float32 данные уже должны быть в диапазоне от -1.0 до 1.0
	// В WAV формате IEEE float они сохраняются без изменений
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// GetBitsPerSample возвращает 32 для 32-битных float данных
func (p *Float32Processor) GetBitsPerSample() uint16 {
	return 32
}

// GetFormatID возвращает 3 (IEEE float) для данных с плавающей точкой
func (p *Float32Processor) GetFormatID() uint16 {
	return 3 // IEEE float формат
}

// GetAudioProcessor возвращает процессор для указанного формата
func GetAudioProcessor(format string) (AudioProcessor, error) {
	switch format {
	case "i8", "int8", "pcm8", "8":
		return &Int8Processor{}, nil
	case "i16", "int16", "pcm16", "16":
		return &Int16Processor{}, nil
	case "i24", "int24", "pcm24", "24":
		return &Int24Processor{}, nil
	case "f32", "float32", "float", "32":
		return &Float32Processor{}, nil
	default:
		return nil, fmt.Errorf("unsupported audio format: %s", format)
	}
}

// GetDefaultProcessor возвращает процессор по умолчанию (8-битный PCM)
func GetDefaultProcessor() AudioProcessor {
	return &Int8Processor{}
}

// ValidateAndGetProcessor проверяет аудиоформат и возвращает соответствующий процессор
// Если формат не указан, использует значение по умолчанию (8-битный PCM)
func ValidateAndGetProcessor(format *AudioFormat) (AudioProcessor, error) {
	if format == nil {
		return GetDefaultProcessor(), nil
	}

	if format.SampleRate == 0 {
		format.SampleRate = 16000 // По умолчанию 16кГц
	}

	// Если формат не указан, используем 8-битный PCM
	if format.Format == "" {
		return GetDefaultProcessor(), nil
	}

	// Получаем процессор для указанного формата
	return GetAudioProcessor(format.Format)
}
