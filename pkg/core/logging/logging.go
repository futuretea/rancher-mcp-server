// Package logging configures structured logging via zerolog with support for
// stdio-safe output (when running as an MCP server) and configurable log levels.
package logging

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	// stdioMode restricts logging to warnings and errors on the configured
	// writer so stdout stays reserved for the MCP protocol.
	stdioMode bool
)

// SetStdioMode switches logging between the stdio and the HTTP/SSE policy.
// In stdio mode only warnings and errors are written, to the given writer:
// they are the only diagnostics a user sees when the server cannot reach
// Rancher. Callers MUST pass a non-stdout writer, because stdout is reserved
// for the MCP protocol. When enabled is false the writer is ignored and the
// current sink is left untouched; call Initialize to configure logging again.
func SetStdioMode(enabled bool, output io.Writer) {
	stdioMode = enabled
	if !enabled {
		return
	}

	if output == nil {
		output = os.Stderr
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// Initialize initializes the global logger with the specified log level and output writer
func Initialize(level int, output io.Writer) {
	// Skip initialization if stdio mode is enabled
	if stdioMode {
		return
	}

	if output == nil {
		output = os.Stderr
	}

	// Set up zerolog with human-friendly console output
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Map our log level (0-9) to zerolog levels
	// 0-1: Error, 2-3: Warn, 4-5: Info, 6+: Debug/Trace
	var zerologLevel zerolog.Level
	switch {
	case level >= 6:
		zerologLevel = zerolog.DebugLevel
	case level >= 4:
		zerologLevel = zerolog.InfoLevel
	case level >= 2:
		zerologLevel = zerolog.WarnLevel
	default:
		zerologLevel = zerolog.ErrorLevel
	}

	zerolog.SetGlobalLevel(zerologLevel)
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	log.Debug().Msgf(format, v...)
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	log.Info().Msgf(format, v...)
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	log.Warn().Msgf(format, v...)
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	log.Error().Msgf(format, v...)
}

// Fatal logs a fatal message and exits
func Fatal(format string, v ...interface{}) {
	log.Fatal().Msgf(format, v...)
}

// GetLogger returns the global zerolog logger for advanced usage
func GetLogger() *zerolog.Logger {
	return &log.Logger
}
