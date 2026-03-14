package logger

import (
	"log/slog"
	"os"
)

var Log *slog.Logger

func Init() {
	if Log != nil {
		return
	}
	// JSON Handler for structured logging (Parity with Winston)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	Log = slog.New(handler)

	// Set as default logger too
	slog.SetDefault(Log)
}

func Info(msg string, args ...any) {
	if Log == nil {
		Init() // Auto-init on usage if missing
	}
	Log.Info(msg, args...)
}

func Error(msg string, args ...any) {
	Log.Error(msg, args...)
}

func Warn(msg string, args ...any) {
	Log.Warn(msg, args...)
}

func Debug(msg string, args ...any) {
	Log.Debug(msg, args...)
}
