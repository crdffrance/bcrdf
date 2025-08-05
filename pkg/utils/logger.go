package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	logLevel = "info"
	logger   *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "", log.LstdFlags)
}

// SetLogLevel définit le niveau de log
func SetLogLevel(level string) {
	logLevel = level
}

// logWithLevel affiche un message selon le niveau de log
func logWithLevel(level, message string) {
	if shouldLog(level) {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		logger.Printf("[%s] %s: %s", timestamp, level, message)
	}
}

// shouldLog détermine si un message doit être affiché selon le niveau de log
func shouldLog(level string) bool {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	currentLevel := levels[logLevel]
	messageLevel := levels[level]

	return messageLevel >= currentLevel
}

// Debug affiche un message de debug
func Debug(format string, args ...interface{}) {
	logWithLevel("DEBUG", fmt.Sprintf(format, args...))
}

// Info affiche un message d'information
func Info(format string, args ...interface{}) {
	logWithLevel("INFO", fmt.Sprintf(format, args...))
}

// Warn affiche un message d'avertissement
func Warn(format string, args ...interface{}) {
	logWithLevel("WARN", fmt.Sprintf(format, args...))
}

// Error affiche un message d'erreur
func Error(format string, args ...interface{}) {
	logWithLevel("ERROR", fmt.Sprintf(format, args...))
}

// Progress affiche une barre de progression
func Progress(current, total int64, operation string) {
	if total == 0 {
		return
	}

	percentage := float64(current) / float64(total) * 100
	Info("%s: %.1f%% (%d/%d)", operation, percentage, current, total)
}
