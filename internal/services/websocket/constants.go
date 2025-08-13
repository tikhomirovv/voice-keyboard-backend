package websocket

import "time"

// WebSocket константы
const (
	// Размеры буферов для WebSocket соединения
	WebSocketReadBufferSize  = 1024
	WebSocketWriteBufferSize = 1024

	// Таймауты
	GracefulCloseTimeout     = 2 * time.Second
	SubscriptionCheckTimeout = 5 * time.Second

	// Сообщения для закрытия соединения
	CloseMessageText = "closing"

	// Коды ошибок
	ErrorCodeParseError         = "PARSE_ERROR"
	ErrorCodeSessionError       = "SESSION_ERROR"
	ErrorCodeUnknownType        = "UNKNOWN_TYPE"
	ErrorCodeDataError          = "DATA_ERROR"
	ErrorCodeProcessingError    = "PROCESSING_ERROR"
	ErrorCodeSubscriptionError  = "SUBSCRIPTION_ERROR"
	ErrorCodeTranscriptionError = "TRANSCRIPTION_ERROR"
	ErrorCodeInternalError      = "INTERNAL_ERROR"
	ErrorCodeFormatError        = "FORMAT_ERROR"

	// Сообщения об ошибках
	ErrorMessageInvalidMessageFormat       = "invalid message format"
	ErrorMessageSessionIDMismatch          = "session ID mismatch"
	ErrorMessageUnknownMessageType         = "unknown message type"
	ErrorMessageEmptyAudioData             = "empty audio data"
	ErrorMessageInvalidAudioData           = "invalid audio data format"
	ErrorMessageFailedToProcessAudio       = "failed to process audio data"
	ErrorMessageFailedToStartSession       = "failed to start session"
	ErrorMessageFailedToStopMessage        = "failed to process stop message"
	ErrorMessageFailedToSendResult         = "failed to send result"
	ErrorMessageFailedToSendError          = "failed to send error message"
	ErrorMessageFailedToCloseSession       = "failed to close session"
	ErrorMessageFailedToSaveAudio          = "failed to save audio data"
	ErrorMessageFailedToDecodeAudio        = "error decoding base64 audio data"
	ErrorMessageAudioFormatError           = "audio format error"
	ErrorMessageFailedToCreateFile         = "failed to create audio file"
	ErrorMessageFailedToCloseFile          = "failed to close audio file"
	ErrorMessageFailedToWriteFile          = "failed to write to audio file"
	ErrorMessageFailedToPrepareRecording   = "failed to prepare for recording"
	ErrorMessageFailedToStartTranscription = "failed to start transcription"
	ErrorMessageFailedToCompleteSession    = "error completing realtime session"
	ErrorMessageFailedToAppendAudio        = "error appending audio to realtime session"
	ErrorMessageFailedToFixText            = "error fixing text"
	ErrorMessageValidSubscriptionRequired  = "valid subscription required to process audio"
	ErrorMessageSubscriptionCheckTimeout   = "subscription check timeout"
	ErrorMessageFailedToCheckSubscription  = "failed to check subscription"
	ErrorMessageUserNoValidSubscription    = "user has no valid subscription"
	ErrorMessageSessionNotStarted          = "session not started"
	ErrorMessageTooManyConnections         = "too many connections for user"
	ErrorMessageSessionAlreadyExists       = "session with this ID already exists"
	ErrorMessageAuthorizationRequired      = "authorization required"
	ErrorMessageInvalidToken               = "invalid token"
	ErrorMessageInvalidRequestBody         = "invalid request body"

	// Форматы аудио
	DefaultAudioFormat = "pcm16"
	DefaultLanguage    = "ru"

	// Заглушки для тестирования
	MockTranscriptionResult = "Пример текста распознавания (из файла)"
)
