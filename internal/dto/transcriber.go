package dto

// TranscriberResult представляет результат транскрибации аудио в текст
type TranscriberResult struct {
	// Text содержит распознанный текст из аудиофайла
	Text string `json:"text"`

	// Duration содержит длительность аудио в секундах
	Duration float64 `json:"duration,omitempty"`

	// LanguageCode содержит код языка распознанного текста (например, "ru-RU")
	LanguageCode string `json:"language_code,omitempty"`

	// Cost содержит стоимость транскрибации
	Cost float64 `json:"cost,omitempty"`
}

// TranscriberRequest представляет запрос на транскрибацию аудио
type TranscriberRequest struct {
	// UserID идентификатор пользователя
	UserID uint64 `json:"user_id"`

	// SessionID идентификатор сессии записи
	SessionID string `json:"session_id"`
}

// RealtimeSessionOptions представляет параметры для создания сессии транскрибации в реальном времени
type RealtimeSessionOptions struct {
	// SessionID идентификатор сессии транскрибации
	SessionID string `json:"session_id" validate:"required"`

	// UserID идентификатор пользователя
	UserID uint64 `json:"user_id" validate:"required"`

	// Format формат аудиоданных (например, "pcm16")
	Format string `json:"format" validate:"required"`

	// Language язык аудио (например, "ru"), если известен
	Language string `json:"language,omitempty"`

	// Prompt подсказка для модели транскрипции, может помочь с точностью
	Prompt string `json:"prompt,omitempty"`
}

// RealtimeAudioData представляет аудиоданные для транскрибации в реальном времени
type RealtimeAudioData struct {
	// SessionID идентификатор сессии
	SessionID string `json:"session_id" validate:"required"`

	// AudioData байты аудиоданных в формате base64
	AudioData []byte `json:"audio_data" validate:"required"`
}

// RealtimeSessionResponse представляет ответ на создание сессии в реальном времени
type RealtimeSessionResponse struct {
	// SessionID идентификатор созданной сессии
	SessionID string `json:"session_id"`

	// Status статус создания сессии
	Status string `json:"status"`
}
