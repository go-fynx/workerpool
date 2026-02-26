// Package tools provides utilities for concurrent task execution and error handling.
//
// WorkerPool Overview:
//
// WorkerPool is a production-ready implementation for managing concurrent job execution
// with controlled resource usage. It maintains a fixed number of worker goroutines that
// process tasks from a buffered queue.
//
// Key Features:
//   - Fixed worker pool for controlled concurrency
//   - Buffered task queue to handle bursts
//   - Automatic timeout enforcement per task
//   - Panic recovery with error reporting
//   - Graceful and immediate shutdown options
//   - Comprehensive metrics tracking
//   - Thread-safe operations
//
// Use Cases:
//   - High-volume API operations
//   - Database batch operations
//   - Email/notification sending
//   - Image/file processing
//   - Rate-limited external API calls
//
// Basic Usage:
//
//	pool, err := tools.NewPoolAndStart(tools.PoolConfig{
//	    Name:        "email-sender",
//	    Workers:     10,
//	    QueueSize:   100,
//	    TaskTimeout: 30 * time.Second,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.ShutdownGraceful(context.Background())
//
//	// Submit tasks
//	err = pool.Submit(tools.Task(func(ctx context.Context) {
//	    sendEmail(ctx, user)
//	}, "jobName", "send-email", "userID", 123))
//
// For detailed documentation, examples, and best practices, see README.md
package workerpool

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool provides a production-ready worker pool for managing multiple concurrent jobs.
// It handles task execution with timeout enforcement, panic recovery, graceful shutdown,
// and comprehensive metrics tracking.
//
// Example usage:
//
//	pool, err := tools.NewWorkerPool(tools.PoolConfig{
//	    Name:        "api-handlers",
//	    Workers:     10,
//	    QueueSize:   100,
//	    TaskTimeout: 5 * time.Second,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	if err := pool.Start(); err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.ShutdownGraceful(context.Background())
//
//	// Submit tasks (same info pattern as AsyncJob)
//	err = pool.Submit(tools.Task(func(ctx context.Context) {
//	    // Your work here
//	}, "jobName", "process-user-data", "userID", 123))

// task represents a unit of work to be executed by the worker pool.
// It follows the same pattern as AsyncJob for consistent developer experience.
type task struct {
	Function func(ctx context.Context)
	info     []any
}

// Task creates a new task with the given function and optional info for logging.
// Similar to AsyncJob, info is used for logging and diagnostics.
//
// Parameters:
//   - function: The work to be performed, must respect context cancellation
//   - info: Optional metadata for logging (key-value pairs recommended)
//
// Example:
//
//	tools.Task(func(ctx context.Context) {
//	    sendEmail(ctx, user)
//	}, "jobName", "emailDispatch", "userID", 123)
func Task(function func(ctx context.Context), info ...any) task {
	if function == nil {
		panic("task function cannot be nil")
	}
	return task{
		Function: function,
		info:     info,
	}
}

// WorkerPool manages a pool of workers that execute tasks concurrently.
// It provides production-ready features including timeout enforcement,
// panic recovery, graceful shutdown, and metrics tracking.
type WorkerPool struct {
	name string

	workers int
	queue   chan task

	ctx    context.Context
	cancel context.CancelFunc

	wg      sync.WaitGroup
	closing atomic.Bool
	started atomic.Bool

	// Metrics - use atomic operations for thread-safe access
	tasksCompleted atomic.Int64
	tasksTimedOut  atomic.Int64
	tasksPanicked  atomic.Int64

	// Configuration
	taskTimeout   time.Duration
	logger        Logger
	errorNotifier ErrorNotifier
}

// NewWorkerPool creates and initializes a new WorkerPool with the given configuration.
// It validates all configuration parameters and returns an error if any are invalid.
//
// The pool must be started with Start() before submitting tasks.
//
// Configuration Guidelines:
//   - Workers: Number of concurrent workers (1-1000 recommended)
//   - QueueSize: Task queue buffer (typically 5-10x workers)
//   - TaskTimeout: Max duration per task (default from config)
//   - Name: Pool identifier for logging (auto-generated if empty)
//
// Example:
//
//	pool, err := NewWorkerPool(PoolConfig{
//	    Name:        "email-sender",
//	    Workers:     5,
//	    QueueSize:   50,
//	    TaskTimeout: 10 * time.Second,
//	})
//
// Returns an error if:
//   - Workers <= 0 or > 1000
//   - QueueSize <= 0 or > 10000
//   - TaskTimeout < 0
func NewWorkerPool(cfg PoolConfig) (*WorkerPool, error) {
	// Validation with reasonable limits
	if cfg.Workers <= 0 {
		return nil, fmt.Errorf("workers must be > 0, got %d", cfg.Workers)
	}
	if cfg.Workers > 1000 {
		return nil, fmt.Errorf("workers must be <= 1000 (got %d), use multiple pools for higher concurrency", cfg.Workers)
	}

	if cfg.QueueSize <= 0 {
		return nil, fmt.Errorf("queueSize must be > 0, got %d", cfg.QueueSize)
	}
	// Set logger default
	if cfg.Logger == nil {
		cfg.Logger = NewDefaultLogger(LogLevelInfo)
	}

	if cfg.QueueSize > 10000 {
		cfg.Logger.Warn(fmt.Sprintf("Large queue size (%d) may consume significant memory", cfg.QueueSize))
	}

	if cfg.TaskTimeout < 0 {
		return nil, fmt.Errorf("taskTimeout cannot be negative, got %v", cfg.TaskTimeout)
	}

	// Set defaults
	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("worker-pool-%d", time.Now().UnixNano())
	}
	if cfg.TaskTimeout == 0 {
		cfg.TaskTimeout = DefaultTaskTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		name:          cfg.Name,
		workers:       cfg.Workers,
		queue:         make(chan task, cfg.QueueSize),
		ctx:           ctx,
		cancel:        cancel,
		taskTimeout:   cfg.TaskTimeout,
		logger:        cfg.Logger,
		errorNotifier: cfg.ErrorNotifier,
	}

	pool.logger.Debugf(
		"Created worker pool '%s': workers=%d, queue=%d, timeout=%v",
		pool.name, pool.workers, cfg.QueueSize, pool.taskTimeout,
	)

	return pool, nil
}

// Start initializes and starts all workers in the pool.
// This method must be called before submitting any tasks.
// It returns an error if the pool has already been started.
//
// The method is thread-safe and can be called only once per pool instance.
func (p *WorkerPool) Start() error {
	if p.started.Swap(true) {
		return fmt.Errorf("worker pool '%s' already started", p.name)
	}

	p.logger.Debugf("Starting worker pool '%s' with %d workers", p.name, p.workers)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	return nil
}

// worker is the main worker loop that processes tasks from the queue.
// Each worker runs in its own goroutine and handles task execution with
// proper timeout and panic recovery.
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	p.logger.Debugf("Worker %d of pool '%s' started", id, p.name)

	for {
		select {
		case task, ok := <-p.queue:
			if !ok {
				// Queue closed → ShutdownGraceful path
				p.logger.Debugf("Worker %d of pool '%s' stopped (queue closed)", id, p.name)
				return
			}
			p.runTask(task, id)

		case <-p.ctx.Done():
			// Context cancelled → Shutdown path (force exit)
			p.logger.Debugf("Worker %d of pool '%s' stopped (context cancelled)", id, p.name)
			return
		}
	}
}

// runTask executes a single task within the worker pool, providing per-task context management,
// panic recovery, statistics gathering, and event logging.
//
// The function ensures that each task is executed with the worker pool's configured timeout, recovers and logs
// panics, and records completion, timeout, or cancellation events.
//
// Args:
//
//	task     - The task to execute, encapsulating the function and optional log context.
//	workerID - The integer ID of the worker processing this task.
//
// Execution flow:
//  1. Sets up a deferred recovery function to handle panics, incrementing the panicked counter and emitting diagnostics.
//  2. Creates a per-task context with timeout derived from the pool.
//  3. Calls the user-supplied task function with the scoped context.
//  4. Records execution duration, discriminates between success, timeout, and cancellation,
//     and updates stats/logs accordingly.
func (p *WorkerPool) runTask(task task, workerID int) {
	start := time.Now()

	// Recover from any panic, log, and increment statistics
	defer p.recoverWorkerPoolPanic(workerID, start, task.info...)

	// Establish a timeout context (inherited from pool settings)
	taskCtx, cancel := context.WithTimeout(p.ctx, p.taskTimeout)
	defer cancel()

	// Invoke the task
	task.Function(taskCtx)

	duration := time.Since(start)

	switch err := taskCtx.Err(); err {
	case nil:
		p.tasksCompleted.Add(1)
		p.logWorkerPoolTask(workerID, duration, task.info...)
	case context.DeadlineExceeded:
		p.tasksTimedOut.Add(1)
		p.logWorkerPoolTimeout(workerID, duration, task.info...)
	default:
		p.tasksTimedOut.Add(1)
		p.logWorkerPoolCancelled(workerID, duration, task.info...)
	}
}

func (p *WorkerPool) recoverWorkerPoolPanic(workerID int, start time.Time, info ...any) {
	err := recover()
	if err == nil {
		return
	}

	p.tasksPanicked.Add(1)
	taskInfo := make([]any, 0, len(info)+4)
	taskInfo = append(taskInfo, "pool", p.name, "worker", workerID)
	taskInfo = append(taskInfo, info...)

	metadata := map[string]any{
		"pool":     p.name,
		"worker":   workerID,
		"info":     info,
		"duration": time.Since(start),
	}

	// Log locally
	p.logger.Errorf("Panic recovered: %v | metadata: %+v", err, metadata)

	// Notify external error tracker if configured
	if p.errorNotifier != nil {
		p.errorNotifier.NotifyPanic(err, metadata)
	}

	p.logWorkerPoolTask(workerID, time.Since(start), info...)
}

// Submit attempts to add a task to the pool's queue without blocking.
// It returns immediately if the queue is full.
//
// Returns an error if:
//   - The pool is shutting down
//   - The pool has not been started
//   - The queue is full
//
// Use [WorkerPool.SubmitBlocking] if you want to wait for queue space to become available.
func (p *WorkerPool) Submit(task task) error {
	if p.closing.Load() {
		return fmt.Errorf("worker pool '%s' is shutting down", p.name)
	}

	if !p.started.Load() {
		return fmt.Errorf("worker pool '%s' not started", p.name)
	}

	select {
	case p.queue <- task:
		return nil
	default:
		return fmt.Errorf("worker pool '%s' queue is full (capacity: %d)", p.name, cap(p.queue))
	}
}

// SubmitBlocking adds a task to the pool's queue, blocking until space is available
// or the provided context is cancelled.
//
// This method is useful when you want to ensure the task is queued but also want
// to respect a deadline or cancellation signal.
//
// Returns an error if:
//   - The pool is shutting down
//   - The pool has not been started
//   - The context is cancelled before the task can be queued
func (p *WorkerPool) SubmitBlocking(ctx context.Context, task task) error {
	if p.closing.Load() {
		return fmt.Errorf("worker pool '%s' is shutting down", p.name)
	}

	if !p.started.Load() {
		return fmt.Errorf("worker pool '%s' not started", p.name)
	}

	select {
	case p.queue <- task:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("worker pool '%s' submission cancelled: %w", p.name, ctx.Err())
	}
}

// ShutdownGraceful initiates a graceful shutdown of the worker pool.
// It closes the queue to prevent new task submissions and waits for all
// currently queued tasks to complete.
//
// The provided context can be used to enforce a shutdown timeout. If the
// context expires before all tasks complete, workers are forcefully stopped
// and any remaining queued tasks are abandoned.
//
// This method is idempotent - calling it multiple times is safe.
func (p *WorkerPool) ShutdownGraceful(ctx context.Context) error {
	if p.closing.Swap(true) {
		// Already shutting down
		return nil
	}

	p.logger.Debugf("Worker pool '%s' initiating graceful shutdown...", p.name)

	// Close queue to prevent new submissions
	close(p.queue)

	// Wait for all tasks to be processed
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Debugf("Worker pool '%s' shutdown gracefully", p.name)
		return nil
	case <-ctx.Done():
		// Force shutdown if timeout
		p.cancel()
		p.logger.Warn(fmt.Sprintf("Worker pool '%s' forced shutdown due to timeout", p.name))
		return fmt.Errorf("graceful shutdown timeout: %w", ctx.Err())
	}
}

// Shutdown forces immediate shutdown of the worker pool.
// It cancels the pool's context, causing all workers to stop immediately.
// Any tasks currently in the queue will be abandoned.
//
// Use this method when you need to stop the pool urgently and cannot wait
// for queued tasks to complete. For a cleaner shutdown, prefer ShutdownGraceful.
//
// This method is idempotent - calling it multiple times is safe.
func (p *WorkerPool) Shutdown(ctx context.Context) error {
	if p.closing.Swap(true) {
		// Already shutting down
		return nil
	}

	p.logger.Debug(fmt.Sprintf("Worker pool '%s' initiating immediate shutdown", p.name))

	// Close queue to signal shutdown
	close(p.queue)

	// Cancel context to stop all workers immediately
	p.cancel()

	// Wait for workers to finish current tasks
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Debugf("Worker pool '%s' shutdown complete", p.name)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown wait timeout: %w", ctx.Err())
	}
}

// Stats returns the current statistics of the worker pool.
// These metrics can be used for monitoring, alerting, and performance analysis.
//
// Returned metrics:
//   - name: Pool identifier
//   - workers: Number of worker goroutines
//   - queue_depth: Current number of tasks in queue
//   - queue_capacity: Maximum queue capacity
//   - queue_utilization: Percentage of queue capacity used (0-100)
//   - tasks_completed: Total number of successfully completed tasks
//   - tasks_timed_out: Total number of tasks that exceeded timeout
//   - tasks_panicked: Total number of tasks that panicked
//   - is_closing: Whether the pool is shutting down
//   - is_started: Whether the pool has been started
//
// Note: This method provides cumulative metrics. Use ResetStats() if you need
// to track metrics over specific time windows.
func (p *WorkerPool) Stats() map[string]any {
	queueDepth := len(p.queue)
	queueCap := cap(p.queue)
	utilization := 0.0
	if queueCap > 0 {
		utilization = float64(queueDepth) / float64(queueCap) * 100
	}

	return map[string]any{
		"name":              p.name,
		"workers":           p.workers,
		"queue_depth":       queueDepth,
		"queue_capacity":    queueCap,
		"queue_utilization": fmt.Sprintf("%.1f%%", utilization),
		"tasks_completed":   p.tasksCompleted.Load(),
		"tasks_timed_out":   p.tasksTimedOut.Load(),
		"tasks_panicked":    p.tasksPanicked.Load(),
		"is_closing":        p.closing.Load(),
		"is_started":        p.started.Load(),
	}
}

// ResetStats resets all cumulative statistics counters to zero.
// This is useful for tracking metrics over specific time windows.
//
// Note: This does not reset queue depth or state flags (is_closing, is_started).
func (p *WorkerPool) ResetStats() {
	p.tasksCompleted.Store(0)
	p.tasksTimedOut.Store(0)
	p.tasksPanicked.Store(0)
	p.logger.Debugf("Worker pool '%s' statistics reset", p.name)
}

// IsHealthy returns true if the pool is running and accepting tasks.
// A pool is considered healthy if it has been started and is not shutting down.
func (p *WorkerPool) IsHealthy() bool {
	return p.started.Load() && !p.closing.Load()
}

// QueueDepth returns the current number of tasks waiting in the queue.
func (p *WorkerPool) QueueDepth() int {
	return len(p.queue)
}

// Name returns the name of the worker pool.
func (p *WorkerPool) Name() string {
	return p.name
}

// Convenience Functions

// NewPoolAndStart creates a new worker pool with the given configuration
// and immediately starts it. This is a convenience function that combines
// NewWorkerPool and Start into a single call.
//
// Example:
//
//	pool, err := tools.NewPoolAndStart(tools.PoolConfig{
//	    Name:        "api-tasks",
//	    Workers:     10,
//	    QueueSize:   100,
//	    TaskTimeout: 5 * time.Second,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pool.ShutdownGraceful(context.Background())
func NewPoolAndStart(cfg PoolConfig) (*WorkerPool, error) {
	pool, err := NewWorkerPool(cfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Start(); err != nil {
		return nil, err
	}

	return pool, nil
}

// SubmitWithRetry attempts to submit a task to the pool with retries.
// If the queue is full, it will retry up to maxRetries times with a backoff delay.
//
// Parameters:
//   - ctx: Context for cancellation
//   - task: The task to submit
//   - maxRetries: Maximum number of retry attempts (0 means no retries)
//   - backoff: Duration to wait between retries
//
// Returns an error if all retries are exhausted or context is cancelled.
func (p *WorkerPool) SubmitWithRetry(ctx context.Context, task task, maxRetries int, backoff time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("submit cancelled during retry: %w", ctx.Err())
			}
		}

		err := p.Submit(task)
		if err == nil {
			if attempt > 0 {
				// Log retry success with task info
				logMsg := make([]any, 0, len(task.info)+5)
				logMsg = append(logMsg, "Task submitted to pool", p.name, "after", attempt, "retries |")
				logMsg = append(logMsg, task.info...)
				p.logger.Info(logMsg...)
			}
			return nil
		}

		lastErr = err

		// If pool is closing or not started, don't retry
		if p.closing.Load() || !p.started.Load() {
			return lastErr
		}
	}

	return fmt.Errorf("failed to submit task after %d retries: %w", maxRetries, lastErr)
}

// Logging functions for WorkerPool tasks (following AsyncJob pattern)

// logWorkerPoolTask logs successful task completion with duration and context info.
// Uses the same format as AsyncJob for consistency.
//
// Log format: Worker-Pool | pool-name | duration | info...
// Example: INFO  2026/02/20 14:46:04 Worker-Pool | email-sender | 3.012984541s | jobName emailDispatch userID 42
func (p *WorkerPool) logWorkerPoolTask(workerID int, duration time.Duration, info ...any) {
	// Build log message header similar to AsyncJob
	jobLog := "Worker-Pool"

	// Construct full log message with pool name, duration, and any supplied context
	// Format: Worker-Pool | pool-name | duration | info...
	logMsg := make([]any, 0, len(info)+5)
	logMsg = append(logMsg, jobLog, "|", p.name, "|", duration, "|")
	logMsg = append(logMsg, info...)

	p.logger.Info(logMsg...)
}

// logWorkerPoolTimeout logs when a task exceeds its timeout.
//
// Log format: Worker-Pool TIMEOUT | pool-name | duration exceeded timeout | info...
func (p *WorkerPool) logWorkerPoolTimeout(workerID int, duration time.Duration, info ...any) {
	jobLog := "Worker-Pool TIMEOUT"

	// Format: Worker-Pool TIMEOUT | pool-name | duration exceeded timeout | info...
	logMsg := make([]any, 0, len(info)+7)
	logMsg = append(logMsg, jobLog, "|", p.name, "|", duration, "|", "exceeded", p.taskTimeout, "|")
	logMsg = append(logMsg, info...)

	p.logger.Warn(logMsg...)
}

// logWorkerPoolCancelled logs when a task is cancelled (usually during shutdown).
//
// Log format: Worker-Pool CANCELLED | pool-name | duration | info...
func (p *WorkerPool) logWorkerPoolCancelled(workerID int, duration time.Duration, info ...any) {
	jobLog := "Worker-Pool CANCELLED"

	// Format: Worker-Pool CANCELLED | pool-name | duration | info...
	logMsg := make([]any, 0, len(info)+5)
	logMsg = append(logMsg, jobLog, "|", p.name, "|", duration, "|")
	logMsg = append(logMsg, info...)

	p.logger.Info(logMsg...)
}
