// Package utils provides utility functions used throughout the application.
package utils

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger that provides structured logging functionality.
type Logger struct {
	*zap.Logger
}

// LoggerOptions configures the logger instance.
type LoggerOptions struct {
	// Development sets the logger to development mode, which makes it more human-readable
	Development bool
	// Level sets the minimum enabled logging level
	Level zapcore.Level
	// OutputPaths defines where logs are written (e.g., stdout, file)
	OutputPaths []string
	// ErrorOutputPaths defines where errors are written
	ErrorOutputPaths []string
}

// DefaultLoggerOptions returns the default logger configuration.
func DefaultLoggerOptions() LoggerOptions {
	return LoggerOptions{
		Development:      false,
		Level:            zapcore.DebugLevel,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// NewLogger creates a new structured logger with the provided options.
// If no options are provided, default options are used.
func NewLogger(opts ...LoggerOptions) *Logger {
	// Use default options if none provided
	options := DefaultLoggerOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	// Create the logger configuration
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(options.Level),
		Development: options.Development,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      options.OutputPaths,
		ErrorOutputPaths: options.ErrorOutputPaths,
	}

	// If in development mode, use a more readable console output
	if options.Development {
		config.Encoding = "console"
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		config.Development = true
	}

	// Build the logger
	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		// If we can't create the logger, use a simple fallback and log the error
		fallback := zap.NewExample()
		fallback.Error("Failed to create logger", zap.Error(err))
		return &Logger{fallback}
	}

	return &Logger{logger}
}

// Info logs a message at info level with structured context.
func (l *Logger) Info(msg string, fields ...any) {
	l.Logger.Info(msg, toZapFields(fields)...)
}

// Error logs a message at error level with structured context.
func (l *Logger) Error(msg string, err error, fields ...any) {
	zapFields := toZapFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.Logger.Error(msg, zapFields...)
}

// Warn logs a message at warn level with structured context.
func (l *Logger) Warn(msg string, fields ...any) {
	l.Logger.Warn(msg, toZapFields(fields)...)
}

// Debug logs a message at debug level with structured context.
func (l *Logger) Debug(msg string, fields ...any) {
	l.Logger.Debug(msg, toZapFields(fields)...)
}

// Fatal logs a message at fatal level with structured context and then calls os.Exit(1).
func (l *Logger) Fatal(msg string, err error, fields ...any) {
	zapFields := toZapFields(fields)
	if err != nil {
		zapFields = append(zapFields, zap.Error(err))
	}
	l.Logger.Fatal(msg, zapFields...)
	// Note: zap.Logger.Fatal already calls os.Exit(1)
}

// With creates a new Logger with additional structured context.
func (l *Logger) With(fields ...any) *Logger {
	return &Logger{l.Logger.With(toZapFields(fields)...)}
}

// Named adds a sub-scope to the logger's name.
func (l *Logger) Named(name string) *Logger {
	return &Logger{l.Logger.Named(name)}
}

// Sync flushes any buffered log entries. Applications should take care to call
// Sync before exiting.
func (l *Logger) Sync() {
	_ = l.Logger.Sync() // Best effort, ignoring errors
}

// toZapFields converts a variadic list of key-value pairs to zap.Field objects.
// It accepts a variadic list of interfaces that must come in pairs:
// odd elements are keys (must be strings) and even elements are values.
func toZapFields(fields []any) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	// If odd number of fields, add a placeholder value
	if len(fields)%2 != 0 {
		fields = append(fields, "MISSING_VALUE")
	}

	result := make([]zap.Field, 0, len(fields)/2)
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			key = "INVALID_KEY"
		}

		value := fields[i+1]
		switch v := value.(type) {
		case string:
			result = append(result, zap.String(key, v))
		case int:
			result = append(result, zap.Int(key, v))
		case int64:
			result = append(result, zap.Int64(key, v))
		case float64:
			result = append(result, zap.Float64(key, v))
		case bool:
			result = append(result, zap.Bool(key, v))
		case error:
			result = append(result, zap.Error(v))
		default:
			result = append(result, zap.Any(key, v))
		}
	}
	return result
}

// GlobalLogger is the default application logger instance
var GlobalLogger *Logger

// Initialize the global logger
func init() {
	// Check if running in development mode
	isDevelopment := os.Getenv("APP_ENV") != "production"

	// Create default logger
	GlobalLogger = NewLogger(LoggerOptions{
		Development: isDevelopment,
		Level:       zapcore.InfoLevel,
	})
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	return GlobalLogger
}
