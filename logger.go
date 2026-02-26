package workerpool

import (
	"log"
	"os"
)

// Logger defines the interface for logging in the worker pool.
// Users can implement this interface to use their own logger.
//
// Example with custom logger:
//
//	type MyLogger struct {
//	    logger *zap.Logger
//	}
//
//	func (l *MyLogger) Debug(args ...any) {
//	    l.logger.Debug(fmt.Sprint(args...))
//	}
//
//	pool, _ := NewWorkerPool(PoolConfig{
//	    Logger: &MyLogger{logger: zapLogger},
//	})
type Logger interface {
	// Debug logs debug-level messages
	Debug(args ...any)

	// Debugf logs formatted debug-level messages
	Debugf(format string, args ...any)

	// Info logs info-level messages
	Info(args ...any)

	// Infof logs formatted info-level messages
	Infof(format string, args ...any)

	// Warn logs warning-level messages
	Warn(args ...any)

	// Warnf logs formatted warning-level messages
	Warnf(format string, args ...any)

	// Error logs error-level messages
	Error(args ...any)

	// Errorf logs formatted error-level messages
	Errorf(format string, args ...any)
}

// LogLevel represents the logging level
type LogLevel int

const (
	// LogLevelDebug enables all log messages
	LogLevelDebug LogLevel = iota
	// LogLevelInfo enables info, warn, and error messages
	LogLevelInfo
	// LogLevelWarn enables warn and error messages
	LogLevelWarn
	// LogLevelError enables only error messages
	LogLevelError
	// LogLevelSilent disables all logging
	LogLevelSilent
)

// DefaultLogger is a simple logger implementation using Go's standard log package.
// It respects the configured log level.
type DefaultLogger struct {
	info  *log.Logger
	debug *log.Logger
	warn  *log.Logger
	error *log.Logger
	level LogLevel
}

// NewDefaultLogger creates a new DefaultLogger with the specified log level.
//
// Example:
//
//	logger := NewDefaultLogger(LogLevelInfo)
//	pool, _ := NewWorkerPool(PoolConfig{
//	    Logger: logger,
//	})
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{
		info:  log.New(os.Stdout, "INFO  ", log.LstdFlags),
		debug: log.New(os.Stdout, "DEBUG ", log.LstdFlags),
		warn:  log.New(os.Stdout, "WARN  ", log.LstdFlags),
		error: log.New(os.Stdout, "ERROR ", log.LstdFlags),
		level: level,
	}
}

// Debug logs debug-level messages
func (l *DefaultLogger) Debug(args ...any) {
	if l.level <= LogLevelDebug {
		l.debug.Println(args...)
	}
}

// Debugf logs formatted debug-level messages
func (l *DefaultLogger) Debugf(format string, args ...any) {
	if l.level <= LogLevelDebug {
		l.debug.Printf(format, args...)
	}
}

// Info logs info-level messages
func (l *DefaultLogger) Info(args ...any) {
	if l.level <= LogLevelInfo {
		l.info.Println(args...)
	}
}

// Infof logs formatted info-level messages
func (l *DefaultLogger) Infof(format string, args ...any) {
	if l.level <= LogLevelInfo {
		l.info.Printf(format, args...)
	}
}

// Warn logs warning-level messages
func (l *DefaultLogger) Warn(args ...any) {
	if l.level <= LogLevelWarn {
		l.warn.Println(args...)
	}
}

// Warnf logs formatted warning-level messages
func (l *DefaultLogger) Warnf(format string, args ...any) {
	if l.level <= LogLevelWarn {
		l.warn.Printf(format, args...)
	}
}

// Error logs error-level messages
func (l *DefaultLogger) Error(args ...any) {
	if l.level <= LogLevelError {
		l.error.Println(args...)
	}
}

// Errorf logs formatted error-level messages
func (l *DefaultLogger) Errorf(format string, args ...any) {
	if l.level <= LogLevelError {
		l.error.Printf(format, args...)
	}
}

// NoOpLogger is a logger that does nothing.
// Use this when you want to disable all logging.
type NoOpLogger struct{}

// NewNoOpLogger creates a new NoOpLogger
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

func (n *NoOpLogger) Debug(args ...any)                 {}
func (n *NoOpLogger) Debugf(format string, args ...any) {}
func (n *NoOpLogger) Info(args ...any)                  {}
func (n *NoOpLogger) Infof(format string, args ...any)  {}
func (n *NoOpLogger) Warn(args ...any)                  {}
func (n *NoOpLogger) Warnf(format string, args ...any)  {}
func (n *NoOpLogger) Error(args ...any)                 {}
func (n *NoOpLogger) Errorf(format string, args ...any) {}
