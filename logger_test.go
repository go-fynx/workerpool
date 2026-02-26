package workerpool

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockLogger is a test logger that captures log messages
type MockLogger struct {
	mu         sync.Mutex
	debugLogs  []string
	infoLogs   []string
	warnLogs   []string
	errorLogs  []string
	debugCount int
	infoCount  int
	warnCount  int
	errorCount int
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		debugLogs: make([]string, 0),
		infoLogs:  make([]string, 0),
		warnLogs:  make([]string, 0),
		errorLogs: make([]string, 0),
	}
}

func (m *MockLogger) Debug(args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugCount++
}

func (m *MockLogger) Debugf(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.debugCount++
}

func (m *MockLogger) Info(args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCount++
}

func (m *MockLogger) Infof(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infoCount++
}

func (m *MockLogger) Warn(args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnCount++
}

func (m *MockLogger) Warnf(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnCount++
}

func (m *MockLogger) Error(args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
}

func (m *MockLogger) Errorf(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount++
}

func (m *MockLogger) GetCounts() (debug, info, warn, error int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.debugCount, m.infoCount, m.warnCount, m.errorCount
}

func TestCustomLogger(t *testing.T) {
	mockLogger := NewMockLogger()

	pool, err := NewWorkerPool(PoolConfig{
		Name:        "test-pool",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 1 * time.Second,
		Logger:      mockLogger,
	})

	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit a task
	err = pool.Submit(Task(func(ctx context.Context) {
		time.Sleep(100 * time.Millisecond)
	}, "jobName", "test-task"))

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for task to complete
	time.Sleep(500 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.ShutdownGraceful(ctx); err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	// Check that logs were captured
	debug, info, warn, _ := mockLogger.GetCounts()

	if debug == 0 {
		t.Error("Expected debug logs but got none")
	}

	if info == 0 {
		t.Error("Expected info logs but got none")
	}

	t.Logf("Log counts - Debug: %d, Info: %d, Warn: %d", debug, info, warn)
}

func TestNoOpLogger(t *testing.T) {
	pool, err := NewWorkerPool(PoolConfig{
		Name:        "noop-pool",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 1 * time.Second,
		Logger:      NewNoOpLogger(),
	})

	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit a task
	err = pool.Submit(Task(func(ctx context.Context) {
		time.Sleep(100 * time.Millisecond)
	}, "jobName", "test-task"))

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for task to complete
	time.Sleep(500 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.ShutdownGraceful(ctx); err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	t.Log("NoOpLogger test completed successfully (no logs expected)")
}

func TestDefaultLogger(t *testing.T) {
	// Test all log levels
	logLevels := []LogLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
		LogLevelSilent,
	}

	for _, level := range logLevels {
		t.Run("LogLevel_"+string(rune(level)), func(t *testing.T) {
			logger := NewDefaultLogger(level)

			pool, err := NewWorkerPool(PoolConfig{
				Name:        "default-logger-pool",
				Workers:     2,
				QueueSize:   5,
				TaskTimeout: 1 * time.Second,
				Logger:      logger,
			})

			if err != nil {
				t.Fatalf("Failed to create pool: %v", err)
			}

			if err := pool.Start(); err != nil {
				t.Fatalf("Failed to start pool: %v", err)
			}

			// Submit a task
			err = pool.Submit(Task(func(ctx context.Context) {
				time.Sleep(50 * time.Millisecond)
			}, "jobName", "test-task"))

			if err != nil {
				t.Fatalf("Failed to submit task: %v", err)
			}

			// Wait for task to complete
			time.Sleep(300 * time.Millisecond)

			// Shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := pool.ShutdownGraceful(ctx); err != nil {
				t.Fatalf("Failed to shutdown pool: %v", err)
			}

			t.Logf("DefaultLogger with level %d completed successfully", level)
		})
	}
}

func TestLoggerNilDefault(t *testing.T) {
	// When no logger is provided, it should use default
	pool, err := NewWorkerPool(PoolConfig{
		Name:        "nil-logger-pool",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 1 * time.Second,
		Logger:      nil, // No logger provided
	})

	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	if pool.logger == nil {
		t.Fatal("Expected default logger to be set, but got nil")
	}

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit a task
	err = pool.Submit(Task(func(ctx context.Context) {
		time.Sleep(50 * time.Millisecond)
	}, "jobName", "test-task"))

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Wait and shutdown
	time.Sleep(300 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.ShutdownGraceful(ctx); err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	t.Log("Nil logger test completed (default logger used)")
}

func TestLoggerTaskTimeout(t *testing.T) {
	mockLogger := NewMockLogger()

	pool, err := NewWorkerPool(PoolConfig{
		Name:        "timeout-test-pool",
		Workers:     1,
		QueueSize:   5,
		TaskTimeout: 100 * time.Millisecond, // Short timeout
		Logger:      mockLogger,
	})

	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}

	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start pool: %v", err)
	}

	// Submit a task that takes longer than timeout
	err = pool.Submit(Task(func(ctx context.Context) {
		time.Sleep(500 * time.Millisecond) // Will timeout
	}, "jobName", "timeout-task"))

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Wait for task to timeout
	time.Sleep(1 * time.Second)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := pool.ShutdownGraceful(ctx); err != nil {
		t.Fatalf("Failed to shutdown pool: %v", err)
	}

	// Check that we got warning logs for timeout
	_, _, warn, _ := mockLogger.GetCounts()

	if warn == 0 {
		t.Error("Expected warning logs for timeout but got none")
	}

	t.Logf("Timeout test completed - Warning logs: %d", warn)
}
