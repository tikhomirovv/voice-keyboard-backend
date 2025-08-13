package websocket

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

// WSSession представляет активную сессию WebSocket
type WSSession struct {
	ID string
	// WS
	UserID            uint64
	Conn              *websocket.Conn
	StartTime         time.Time
	LastActivityTime  time.Time
	Started           bool      // Флаг, указывающий, была ли сессия начата
	subscriptionOnce  sync.Once // Гарантирует однократное выполнение проверки подписки
	subscriptionValid bool      // Результат проверки подписки
	Mutex             sync.Mutex
	AudioOptions      WSSessionAudioOptions
}

type WSSessionAudioOptions struct {
	Format        string
	SampleRate    uint32
	AudioFilePath string   // Путь к временному файлу с аудиоданными
	AudioFile     *os.File // Дескриптор файла для записи аудиоданных
}
