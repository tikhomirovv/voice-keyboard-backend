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
