package logger

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

const (
	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarn    = "warn"
	LogLevelError   = "error"
	DefaultLogLevel = zerolog.DebugLevel
)

type LogLevel string
type ZLogger struct {
	logger zerolog.Logger
}

func NewLogger(logLevel LogLevel) Logger {
	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.TimeFormat = time.RFC3339
	})).
		With().
		Timestamp().
		Logger()

	switch logLevel {
	case LogLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LogLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LogLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LogLevelError:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(DefaultLogLevel)
	}

	return &ZLogger{logger}
}

func (z ZLogger) Debug(msg string, args ...any) {
	z.logger.Debug().Timestamp().Fields(args).Msg(msg)
}

func (z ZLogger) Info(msg string, args ...any) {
	z.logger.Info().Timestamp().Fields(args).Msg(msg)
}

func (z ZLogger) Warn(msg string, args ...any) {
	z.logger.Warn().Timestamp().Fields(args).Msg(msg)
}

func (z ZLogger) Error(msg string, args ...any) {
	z.logger.Error().Timestamp().Fields(args).Msg(msg)
}

func (z ZLogger) Panic(msg string, args ...any) {
	z.logger.Panic().Timestamp().Fields(args).Msg(msg)
}

func (z ZLogger) Printf(format string, v ...any) {
	z.logger.Printf(format, v...)
}

func (z ZLogger) Fatal(v ...any) {
	z.logger.Fatal().Timestamp().Msg(fmt.Sprint(v...))
}

func (z ZLogger) Fatalf(format string, args ...any) {
	z.logger.Fatal().Timestamp().Msgf(format, args...)
}

func (z ZLogger) Println(args ...any) {
	z.logger.Info().Timestamp().Msgf("%v\r\n", args...)
}

func (z ZLogger) Print(args ...any) {
	z.logger.Print(args...)
}
