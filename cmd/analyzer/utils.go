package main

import (
	"fmt"
	"log/slog"
)

// userAgent returns the User-Agent string for HTTP requests
func userAgent() string {
	return fmt.Sprintf("page-analyzer/%s", Version)
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
