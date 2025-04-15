package dto

// WsConnectAudioDTO представляет данные для подключения к аудиосессии
type WsConnectAudioDTO struct {
	SessionID  string `json:"sessionId" validate:"required"`
	Format     string `json:"format" validate:"required"`
	SampleRate uint32 `json:"sampleRate" validate:"required"`
}
