package workerpool

import "time"

// Default configuration values
const (
	DefaultTaskTimeout = 30 * time.Second
)

// PoolConfig holds configuration for creating a new WorkerPool.
type PoolConfig struct {
	// Name identifies the pool in logs and metrics
	Name string

	// Workers is the number of concurrent workers
	Workers int

	// QueueSize is the buffer size for the task queue
	QueueSize int

	// TaskTimeout is the maximum duration for a single task
	// Default: 30 seconds (DefaultTaskTimeout)
	TaskTimeout time.Duration

	// Logger is the logger instance to use for logging
	// Default: NewDefaultLogger(LogLevelInfo)
	Logger Logger

	// ErrorNotifier is an optional error notification handler (e.g., Bugsnag)
	// If nil, errors are only logged locally
	ErrorNotifier ErrorNotifier
}

// DefaultPoolConfig returns a PoolConfig with sensible defaults.
// You can modify the returned config before passing it to NewWorkerPool.
//
// Default values:
//   - Name: provided name
//   - Workers: 5 (conservative default)
//   - QueueSize: Workers * 10
//   - TaskTimeout: 30 seconds
//   - Logger: nil (will use NewDefaultLogger(LogLevelInfo))
//   - ErrorNotifier: nil (no external error reporting)
func DefaultPoolConfig(name string) PoolConfig {
	workers := 5 // Conservative default
	return PoolConfig{
		Name:        name,
		Workers:     workers,
		QueueSize:   workers * 10,
		TaskTimeout: DefaultTaskTimeout,
	}
}

// SetName sets the name of the pool for logging and metrics purposes.
// Returns the modified PoolConfig for chaining.
func (cfg *PoolConfig) SetName(name string) *PoolConfig {
	cfg.Name = name
	return cfg
}

// SetLogger sets the logger instance to use for the pool.
// Returns the modified PoolConfig for chaining.
func (cfg *PoolConfig) SetLogger(logger Logger) *PoolConfig {
	cfg.Logger = logger
	return cfg
}

// SetTaskTimeout sets the maximum duration allowed for a single task in the pool.
// Returns the modified PoolConfig for chaining.
func (cfg *PoolConfig) SetTaskTimeout(taskTimeout time.Duration) *PoolConfig {
	cfg.TaskTimeout = taskTimeout
	return cfg
}

// SetWorkers sets the number of concurrent workers for the pool.
// Returns the modified PoolConfig for chaining.
func (cfg *PoolConfig) SetWorkers(workers int) *PoolConfig {
	cfg.Workers = workers
	return cfg
}

// SetQueueSize sets the buffer size for the pool's task queue.
// Returns the modified PoolConfig for chaining.
func (cfg *PoolConfig) SetQueueSize(queueSize int) *PoolConfig {
	cfg.QueueSize = queueSize
	return cfg
}

// SetErrorNotifier sets the error notification handler for the pool.
// This is typically used for external error tracking services like Bugsnag, Sentry, etc.
// Returns the modified PoolConfig for chaining.
//
// Example:
//
//	notifier := workerpool.NewBugsnagNotifier()
//	config.SetErrorNotifier(notifier)
func (cfg *PoolConfig) SetErrorNotifier(notifier ErrorNotifier) *PoolConfig {
	cfg.ErrorNotifier = notifier
	return cfg
}
