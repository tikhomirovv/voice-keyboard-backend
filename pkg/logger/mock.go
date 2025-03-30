package logger

type LoggerMock struct {
}

func (l *LoggerMock) Debug(msg string, fields ...any) {}
func (l *LoggerMock) Info(msg string, fields ...any)  {}
func (l *LoggerMock) Warn(msg string, fields ...any)  {}
func (l *LoggerMock) Error(msg string, fields ...any) {}
func (l *LoggerMock) Fatal(msg string, fields ...any) {}
func (l *LoggerMock) Panic(msg string, fields ...any) {}
