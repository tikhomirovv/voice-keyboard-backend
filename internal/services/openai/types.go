package openai

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gitlab.com/voice-keyboard/backend-go/internal/dto"
	"gitlab.com/voice-keyboard/backend-go/pkg"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"
)

// Message представляет сообщение для генерации текста в формате OpenAI
type Message struct {
	Role    string `json:"role" validate:"required,oneof=user assistant developer"`
	Content string `json:"content" validate:"required"`
}

// TextGenerationRequest представляет запрос на генерацию текста
type TextGenerationRequest struct {
	// Model - название модели OpenAI для генерации текста
	Model string `json:"model" validate:"required"`

	// Messages - массив сообщений для генерации текста
	Messages []Message `json:"messages" validate:"required,dive"`

	// MaxTokens - максимальное количество токенов в ответе
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature - параметр случайности (0.0-2.0)
	Temperature float32 `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`

	// TopP - параметр вероятности следующего токена (0.0-1.0)
	TopP float32 `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
}

// TextGenerationChoice представляет один вариант сгенерированного текста
type TextGenerationChoice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// TextGenerationResponse представляет ответ от сервиса генерации текста
type TextGenerationResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []TextGenerationChoice `json:"choices"`
	Usage   TextGenerationUsage    `json:"usage"`
}

// TextGenerationUsage представляет информацию об использовании токенов
type TextGenerationUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ResponseRequest представляет запрос к Responses API
type ResponseRequest struct {
	// Model - название модели OpenAI для генерации текста
	Model string `json:"model" validate:"required"`

	// Instructions - инструкции для модели (опционально)
	Instructions string `json:"instructions,omitempty"`

	// Input - входные данные для запроса
	Input string `json:"input" validate:"required"`

	// MaxOutputTokens - максимальное количество токенов в ответе
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
}

type ResponseRequestWithoutReasoning struct {
	ResponseRequest
	Temperature float32 `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`
	// TopP        float32 `json:"top_p,omitempty" validate:"omitempty,min=0,max=1"`
}

type ResponseRequestWithReasoning struct {
	ResponseRequest
	Reasoning RequestReasoningFields `json:"reasoning,omitempty"`
}

type RequestReasoningFields struct {
	// Effort - ограничивает усилия на рассуждения для моделей с рассуждениями
	// Поддерживаемые значения: minimal, low, medium, high
	// Уменьшение усилий может привести к более быстрым ответам и меньшему количеству токенов
	Effort string `json:"effort,omitempty" validate:"omitempty,oneof=minimal low medium high"`
	// Summary - краткое изложение рассуждений, выполненных моделью
	// Полезно для отладки и понимания процесса рассуждений модели
	Summary string `json:"summary,omitempty" validate:"omitempty,oneof=auto concise detailed"`
}

// OutputContent представляет содержимое ответа
type OutputContent struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Annotations []any  `json:"annotations"`
}

// OutputMessage представляет сообщение в ответе от Responses API
type OutputMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Status  string          `json:"status"`
	Role    string          `json:"role"`
	Content []OutputContent `json:"content"`
}

// ResponseUsage представляет информацию об использовании токенов в Responses API
type ResponseUsage struct {
	InputTokens         int            `json:"input_tokens"`
	OutputTokens        int            `json:"output_tokens"`
	TotalTokens         int            `json:"total_tokens"`
	InputTokensDetails  map[string]any `json:"input_tokens_details"`
	OutputTokensDetails map[string]any `json:"output_tokens_details"`
}

// ResponseResult представляет ответ от Responses API
type ResponseResult struct {
	ID                 string          `json:"id"`
	Object             string          `json:"object"`
	CreatedAt          int64           `json:"created_at"`
	Status             string          `json:"status"`
	Error              any             `json:"error"`
	IncompleteDetails  any             `json:"incomplete_details"`
	Instructions       any             `json:"instructions"`
	MaxOutputTokens    any             `json:"max_output_tokens"`
	Model              string          `json:"model"`
	Output             []OutputMessage `json:"output"`
	ParallelToolCalls  bool            `json:"parallel_tool_calls"`
	PreviousResponseID any             `json:"previous_response_id"`
	Store              bool            `json:"store"`
	Temperature        float32         `json:"temperature"`
	Text               map[string]any  `json:"text"`
	ToolChoice         string          `json:"tool_choice"`
	Tools              []any           `json:"tools"`
	TopP               float32         `json:"top_p"`
	Truncation         string          `json:"truncation"`
	Usage              ResponseUsage   `json:"usage"`
	User               any             `json:"user"`
	Metadata           map[string]any  `json:"metadata"`
}

// Константы для работы с Realtime API
const (
	// WebSocket URL для Realtime API с intent=transcription
	realtimeAPIURL = "wss://api.openai.com/v1/realtime"

	// Типы событий от сервера к клиенту
	eventTypeTranscriptionDelta            = "conversation.item.input_audio_transcription.delta"
	eventTypeTranscriptionCompleted        = "conversation.item.input_audio_transcription.completed"
	eventTypeTranscriptionSessionCreated   = "transcription_session.created"
	eventTypeInputAudioBufferCommitted     = "input_audio_buffer.committed"
	eventTypeInputAudioBufferSpeechStarted = "input_audio_buffer.speech_started"
	eventTypeInputAudioBufferSpeechStopped = "input_audio_buffer.speech_stopped"
	eventTypeError                         = "error"

	// Типы событий от клиента к серверу
	eventTypeInputAudioBufferAppend = "input_audio_buffer.append"
	eventTypeInputAudioBufferCommit = "input_audio_buffer.commit"
	eventTypeTranscriptionSession   = "transcription_session.update"

	// Таймауты и интервалы
	sessionInactivityTimeout = 1 * time.Minute // Таймаут неактивности сессии для очистки
	transcriptionWaitTimeout = 5 * time.Second // Таймаут ожидания завершения транскрипции (уменьшил с 30 до 10)
	commitTimeout            = 5 * time.Second // Таймаут ожидания коммита
)

// ItemStatus описывает статус элемента разговора
type ItemStatus int

const (
	ItemSpeechStarted ItemStatus = iota
	ItemSpeechStopped
	ItemCommitted
	ItemCompleted
)

// ConversationItem описывает элемент разговора с его статусом и текстом
type ConversationItem struct {
	ItemID         string     // ID элемента
	PreviousItemID string     // ID предыдущего элемента (для определения порядка)
	Status         ItemStatus // Статус: committed или completed
	Transcript     string     // Текст транскрипции (заполняется при completed)
}

// ConversationItemUpdate описывает опции для обновления элемента разговора
type ConversationItemUpdate struct {
	PreviousItemID *string    // Указатель на previous_item_id (nil = не обновлять)
	Status         ItemStatus // Указатель на статус (nil = не обновлять)
	Transcript     *string    // Указатель на транскрипцию (nil = не обновлять)
}

// RealtimeSession представляет сессию транскрипции с собственным подключением к OpenAI
type RealtimeSession struct {
	// Основная информация о сессии
	ID         string                      // Уникальный идентификатор сессии
	UserID     uint64                      // ID пользователя
	Format     string                      // Формат аудио (pcm16 и т.д.)
	Language   string                      // Язык транскрибации
	Prompt     string                      // Промпт для транскрибации
	ResultCh   chan *dto.TranscriberResult // Канал для отправки результатов транскрипции
	Created    time.Time                   // Время создания сессии
	LastActive time.Time                   // Время последней активности

	// Подключение к OpenAI
	conn      *websocket.Conn // WebSocket соединение
	config    *pkg.Config     // Конфигурация
	logger    logger.Logger   // Логгер
	closed    bool            // Флаг закрытия
	closeCh   chan struct{}   // Канал для сигнала закрытия
	connMutex sync.Mutex      // Мьютекс для соединения
	ready     bool            // Флаг готовности сессии

	// Флаг активности речи и мьютекс для его защиты
	isSpeech    bool
	speechMutex sync.RWMutex

	// Флаг ожидания коммита и канал для события committed
	waitingCommit bool
	commitCh      chan struct{} // Канал для сигнала о получении committed события

	// Карта элементов разговора для отслеживания по item_id
	conversationItems map[string]*ConversationItem
	itemsMutex        sync.RWMutex
}

// RealtimeEvent описывает структуру события от Realtime API
type RealtimeEvent struct {
	EventID        string         `json:"event_id,omitempty"`
	Type           string         `json:"type"`
	ItemID         string         `json:"item_id,omitempty"`
	ContentIndex   int            `json:"content_index,omitempty"`
	Delta          string         `json:"delta,omitempty"`
	Transcript     string         `json:"transcript,omitempty"`
	PreviousItemID string         `json:"previous_item_id,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Error          *RealtimeError `json:"error,omitempty"`
}

// RealtimeError описывает ошибку от Realtime API
type RealtimeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

// AudioBufferAppendEvent описывает событие для отправки аудиоданных
type AudioBufferAppendEvent struct {
	Type     string         `json:"type"`
	Audio    string         `json:"audio"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AudioBufferCommitEvent описывает событие для коммита аудиобуфера
type AudioBufferCommitEvent struct {
	Type string `json:"type"`
}

// TranscriptionSessionUpdateEvent описывает событие для обновления сессии транскрипции
type TranscriptionSessionUpdateEvent struct {
	Type    string        `json:"type"`
	Session SessionObject `json:"session"`
}

type SessionObject struct {
	InputAudioFormat         string                   `json:"input_audio_format"`
	InputAudioTranscription  *InputAudioTranscription `json:"input_audio_transcription"`
	TurnDetection            *TurnDetection           `json:"turn_detection"`
	InputAudioNoiseReduction *NoiseReduction          `json:"input_audio_noise_reduction"`
	Include                  []string                 `json:"include,omitempty"`
}

// InputAudioTranscription описывает настройки транскрипции
type InputAudioTranscription struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt,omitempty"`
	Language string `json:"language,omitempty"`
}

// TurnDetection описывает настройки обнаружения речи
type TurnDetection struct {
	Type string `json:"type"`
	// Threshold         float64 `json:"threshold"`
	PrefixPaddingMS   int `json:"prefix_padding_ms"`
	SilenceDurationMS int `json:"silence_duration_ms"`
	// Eagerness         string `json:"eagerness"` // (semantic_vad) "low" | "medium" | "high" | "auto", // optional
}

// NoiseReduction описывает настройки шумоподавления
type NoiseReduction struct {
	Type string `json:"type"`
}

// RealtimeTranscriberService реализует интерфейс RealtimeTranscriberServiceInterface
// с использованием OpenAI Realtime API для транскрипции аудио в реальном времени
type RealtimeTranscriberService struct {
	config        *pkg.Config
	logger        logger.Logger
	sessionsMutex sync.RWMutex
	sessions      map[string]*RealtimeSession
}
