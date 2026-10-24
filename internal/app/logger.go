package app

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Empty logPath writes only to stdout, as expected in Kubernetes.
func NewLogger(logPath string, level slog.Level) (*slog.Logger, *os.File, error) {
	var output io.Writer = os.Stdout
	var file *os.File
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		var err error
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		output = io.MultiWriter(os.Stdout, file)
	}
	logger := slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger, file, nil
}
