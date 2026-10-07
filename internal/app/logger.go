package app

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func NewLogger() (*slog.Logger, *os.File, error) {
	logPath := os.Getenv("LOG_FILE")
	if logPath == "" {
		return nil, nil, fmt.Errorf("LOG_FILE is required")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}

	handler := slog.NewJSONHandler(
		io.MultiWriter(os.Stdout, file),
		&slog.HandlerOptions{
			Level: slog.LevelInfo,
		},
	)

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, file, nil
}
