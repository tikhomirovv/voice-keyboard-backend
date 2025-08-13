package interfaces

type AudioServiceInterface interface {
	Create(userID uint64, sessionID string, sampleRate uint32) (string, error)
	WriteData(sessionID string, data []byte) error
	Close(sessionID string) (string, error)
	Remove(userID uint64, sessionID string) error
	GetAudioFilePath(userID uint64, sessionID string, isTemp bool) string
}
