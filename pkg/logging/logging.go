package logging

import (
	"log"
	"os"
)

// Logger represents a leveled logger
var Logger *LeveledLogger

// LeveledLogger provides leveled logging functionality
type LeveledLogger struct {
	level int
}

// NewLeveledLogger creates a new leveled logger
func NewLeveledLogger(level int) *LeveledLogger {
	return &LeveledLogger{
		level: level,
	}
}

// SetLevel sets the log level
func (l *LeveledLogger) SetLevel(level int) {
	l.level = level
}

// Debug logs at debug level (level 5-9)
func (l *LeveledLogger) Debug(format string, v ...interface{}) {
	if l.level >= 5 {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs at info level (level 3-9)
func (l *LeveledLogger) Info(format string, v ...interface{}) {
	if l.level >= 3 {
		log.Printf("[INFO] "+format, v...)
	}
}

// Warn logs at warning level (level 1-9)
func (l *LeveledLogger) Warn(format string, v ...interface{}) {
	if l.level >= 1 {
		log.Printf("[WARN] "+format, v...)
	}
}

// Error logs at error level (level 0-9)
func (l *LeveledLogger) Error(format string, v ...interface{}) {
	if l.level >= 0 {
		log.Printf("[ERROR] "+format, v...)
	}
}

// Initialize initializes the global logger
func Initialize(level int) {
	Logger = NewLeveledLogger(level)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}