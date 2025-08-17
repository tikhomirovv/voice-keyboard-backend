package transcribe

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

// MessageType определяет тип сообщения, которое отправляется через WebSocket
type MessageType string

const (
	// Типы сообщений от клиента к серверу
	MessageTypeAudio MessageType = "audio" // Аудиоданные
	MessageTypeStop  MessageType = "stop"  // Конец сессии

	// Типы сообщений от сервера к клиенту
	MessageTypePartial   MessageType = "partial"   // Частичный результат транскрипции
	MessageTypeCompleted MessageType = "completed" // Результат транскрипции
	MessageTypeError     MessageType = "error"     // Ошибка
)

// WebSocketMessage представляет структуру сообщения через WebSocket
type WebSocketMessage struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	Data      json.RawMessage `json:"data"`
}

// AudioData содержит аудиоданные в сообщении
type AudioData struct {
	Samples string `json:"samples"` // Байты аудиосэмплов в base64
}

// CompletedData содержит результат распознавания
type CompletedData struct {
	Text string `json:"text"`
}

// PartialData содержит частичное распознанное текстовое сообщение
type PartialData struct {
	Text string `json:"text"`
}

// ErrorData содержит информацию об ошибке
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WSSession представляет активную сессию WebSocket для Fiber
type WSSession struct {
	ID                string
	UserID            uint64
	Conn              *websocket.Conn
	StartTime         time.Time
	LastActivityTime  time.Time
	Started           bool
	subscriptionOnce  sync.Once
	subscriptionValid bool
	Mutex             sync.Mutex
	AudioOptions      WSSessionAudioOptions
}

type WSSessionAudioOptions struct {
	SampleFormat string // pcm16, i16, i8, f32, i24
	SampleRate   uint32 // 16000, 48000
}
