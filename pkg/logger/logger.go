package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a logging level
type Level int

const (
	// Debug level for detailed debugging information
	Debug Level = iota
	// Info level for general operational information
	Info
	// Warn level for warning messages that might indicate problems
	Warn
	// Error level for error messages
	Error
)

var levelStrings = map[Level]string{
	Debug: "DEBUG",
	Info:  "INFO",
	Warn:  "WARN",
	Error: "ERROR",
}

var stringToLevel = map[string]Level{
	"debug": Debug,
	"info":  Info,
	"warn":  Warn,
	"error": Error,
}

// Logger represents a structured logger
type Logger struct {
	level  Level
	prefix string
	writer io.Writer
	mu     sync.Mutex
}

// New creates a new logger with the specified level and prefix
func New(level string, prefix string) *Logger {
	l, exists := stringToLevel[strings.ToLower(level)]
	if !exists {
		l = Info // Default to Info if invalid level
	}

	return &Logger{
		level:  l,
		prefix: prefix,
		writer: os.Stdout,
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLevel, exists := stringToLevel[strings.ToLower(level)]
	if exists {
		l.level = newLevel
	}
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writer = w
}

// log logs a message at the specified level
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	logEntry := fmt.Sprintf("%s [%s] %s: %s\n", timestamp, levelStrings[level], l.prefix, message)

	_, err := io.WriteString(l.writer, logEntry)
	if err != nil {
		log.Printf("Error writing to log: %v", err)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(Debug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(Info, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(Warn, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(Error, format, args...)
}

// Default logger
var defaultLogger = New("info", "ARR-MCP")

// SetDefaultLevel sets the level of the default logger
func SetDefaultLevel(level string) {
	defaultLogger.SetLevel(level)
}

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}