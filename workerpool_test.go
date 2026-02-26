package workerpool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ExamplePoolConfig demonstrates WorkerPool configuration.
func ExamplePoolConfig() {
	// Configure a worker pool
	config := PoolConfig{
		Name:        "email-sender",
		Workers:     5,
		QueueSize:   50,
		TaskTimeout: 10 * time.Second,
	}

	fmt.Printf("Pool: %s\n", config.Name)
	fmt.Printf("Workers: %d\n", config.Workers)
	fmt.Printf("Queue: %d\n", config.QueueSize)

	// Output:
	// Pool: email-sender
	// Workers: 5
	// Queue: 50
}

// Example code showing WorkerPool API patterns.
// Actual execution examples are in the test files.

// TestNewWorkerPool_Validation tests configuration validation
func TestNewWorkerPool_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  PoolConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: PoolConfig{
				Name:        "test-pool",
				Workers:     5,
				QueueSize:   10,
				TaskTimeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "zero workers",
			config: PoolConfig{
				Name:      "test-pool",
				Workers:   0,
				QueueSize: 10,
			},
			wantErr: true,
			errMsg:  "workers must be > 0",
		},
		{
			name: "negative workers",
			config: PoolConfig{
				Name:      "test-pool",
				Workers:   -1,
				QueueSize: 10,
			},
			wantErr: true,
			errMsg:  "workers must be > 0",
		},
		{
			name: "too many workers",
			config: PoolConfig{
				Name:      "test-pool",
				Workers:   1001,
				QueueSize: 10,
			},
			wantErr: true,
			errMsg:  "workers must be <= 1000",
		},
		{
			name: "zero queue size",
			config: PoolConfig{
				Name:      "test-pool",
				Workers:   5,
				QueueSize: 0,
			},
			wantErr: true,
			errMsg:  "queueSize must be > 0",
		},
		{
			name: "negative timeout",
			config: PoolConfig{
				Name:        "test-pool",
				Workers:     5,
				QueueSize:   10,
				TaskTimeout: -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "taskTimeout cannot be negative",
		},
		{
			name: "auto-generated name",
			config: PoolConfig{
				Workers:   5,
				QueueSize: 10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewWorkerPool(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if pool == nil {
					t.Error("expected pool to be created")
				}
			}
		})
	}
}

// TestWorkerPool_BasicExecution tests basic task execution
func TestWorkerPool_BasicExecution(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "basic-test",
		Workers:     2,
		QueueSize:   15, // Increased to handle all tasks
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	var counter atomic.Int32
	var wg sync.WaitGroup

	// Submit 10 tasks
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		err := pool.SubmitBlocking(ctx, Task(func(ctx context.Context) {
			counter.Add(1)
			wg.Done()
		}, "jobName", "test-task"))

		if err != nil {
			t.Errorf("failed to submit task: %v", err)
			wg.Done()
		}
	}

	// Wait for all tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("tasks did not complete in time")
	}

	if count := counter.Load(); count != 10 {
		t.Errorf("expected 10 tasks executed, got %d", count)
	}

	stats := pool.Stats()
	completed := stats["tasks_completed"].(int64)
	if completed != 10 {
		t.Errorf("expected 10 completed tasks in stats, got %d", completed)
	}
}

// TestWorkerPool_PanicRecovery tests that panics are caught and don't crash
func TestWorkerPool_PanicRecovery(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "panic-test",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Submit task that panics
	err = pool.Submit(Task(func(ctx context.Context) {
		panic("intentional panic for testing")
	}, "jobName", "panic-task"))

	if err != nil {
		t.Errorf("failed to submit task: %v", err)
	}

	// Wait for task to be processed
	time.Sleep(500 * time.Millisecond)

	// Submit another task to verify pool still works
	var executed atomic.Bool
	err = pool.Submit(Task(func(ctx context.Context) {
		executed.Store(true)
	}, "jobName", "after-panic"))

	if err != nil {
		t.Errorf("failed to submit task after panic: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if !executed.Load() {
		t.Error("pool did not execute task after panic")
	}

	stats := pool.Stats()
	panicked := stats["tasks_panicked"].(int64)
	if panicked != 1 {
		t.Errorf("expected 1 panicked task, got %d", panicked)
	}
}

// TestWorkerPool_Timeout tests task timeout enforcement
func TestWorkerPool_Timeout(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "timeout-test",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Submit task that respects context
	err = pool.Submit(Task(func(ctx context.Context) {
		select {
		case <-time.After(2 * time.Second):
			// Should not reach here
		case <-ctx.Done():
			// Context cancelled - timeout respected
			return
		}
	}, "jobName", "timeout-task"))

	if err != nil {
		t.Errorf("failed to submit task: %v", err)
	}

	// Wait for timeout to occur
	time.Sleep(1 * time.Second)

	stats := pool.Stats()
	timedOut := stats["tasks_timed_out"].(int64)
	if timedOut != 1 {
		t.Errorf("expected 1 timed out task, got %d", timedOut)
	}
}

// TestWorkerPool_ContextCancellation tests context cancellation
func TestWorkerPool_ContextCancellation(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "cancel-test",
		Workers:     2,
		QueueSize:   5,
		TaskTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	var cancelled atomic.Bool

	err = pool.Submit(Task(func(ctx context.Context) {
		select {
		case <-time.After(5 * time.Second):
			// Should not reach here
		case <-ctx.Done():
			cancelled.Store(true)
			return
		}
	}, "jobName", "cancel-task"))

	if err != nil {
		t.Errorf("failed to submit task: %v", err)
	}

	// Wait a bit for task to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown pool (cancels all contexts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pool.ShutdownGraceful(ctx)

	time.Sleep(200 * time.Millisecond)

	if !cancelled.Load() {
		t.Error("task context was not cancelled during shutdown")
	}
}

// TestWorkerPool_QueueFull tests queue capacity
func TestWorkerPool_QueueFull(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "queue-full-test",
		Workers:     1,
		QueueSize:   2,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Block the worker
	var workerBlocked atomic.Bool
	err = pool.Submit(Task(func(ctx context.Context) {
		workerBlocked.Store(true)
		time.Sleep(2 * time.Second)
	}, "jobName", "blocking-task"))

	if err != nil {
		t.Errorf("failed to submit blocking task: %v", err)
	}

	// Wait for worker to be blocked
	time.Sleep(100 * time.Millisecond)

	// Fill the queue (2 slots)
	for i := 0; i < 2; i++ {
		err = pool.Submit(Task(func(ctx context.Context) {
			time.Sleep(100 * time.Millisecond)
		}, "jobName", fmt.Sprintf("queue-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// Next submission should fail (queue full)
	err = pool.Submit(Task(func(ctx context.Context) {
		// Should not execute
	}, "jobName", "overflow-task"))

	if err == nil {
		t.Error("expected error when queue is full, got nil")
	} else if !contains(err.Error(), "queue is full") {
		t.Errorf("expected 'queue is full' error, got: %v", err)
	}
}

// TestWorkerPool_SubmitBlocking tests blocking submission
func TestWorkerPool_SubmitBlocking(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "blocking-submit-test",
		Workers:     1,
		QueueSize:   1,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Wait for worker to be ready
	time.Sleep(100 * time.Millisecond)

	// Fill queue and worker
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		err = pool.SubmitBlocking(ctx, Task(func(ctx context.Context) {
			time.Sleep(500 * time.Millisecond)
		}, "jobName", fmt.Sprintf("fill-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// SubmitBlocking should wait and eventually succeed
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var executed atomic.Bool
	err = pool.SubmitBlocking(ctx, Task(func(ctx context.Context) {
		executed.Store(true)
	}, "jobName", "blocking-task"))

	if err != nil {
		t.Errorf("SubmitBlocking failed: %v", err)
	}

	// Wait for execution
	time.Sleep(1 * time.Second)

	if !executed.Load() {
		t.Error("blocking task was not executed")
	}
}

// TestWorkerPool_SubmitBlockingTimeout tests blocking submission timeout
func TestWorkerPool_SubmitBlockingTimeout(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "blocking-timeout-test",
		Workers:     1,
		QueueSize:   1,
		TaskTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Wait for worker to start
	time.Sleep(100 * time.Millisecond)

	// Fill queue and worker with long tasks using SubmitBlocking
	submitCtx := context.Background()
	for i := 0; i < 2; i++ {
		err = pool.SubmitBlocking(submitCtx, Task(func(ctx context.Context) {
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}, "jobName", fmt.Sprintf("long-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// SubmitBlocking with short timeout should fail
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err = pool.SubmitBlocking(ctx, Task(func(ctx context.Context) {
		// Should not execute
	}, "jobName", "timeout-task"))

	if err == nil {
		t.Error("expected timeout error, got nil")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}

// TestWorkerPool_SubmitWithRetry tests retry logic
func TestWorkerPool_SubmitWithRetry(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "retry-test",
		Workers:     1,
		QueueSize:   1,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Wait for worker to be ready
	time.Sleep(100 * time.Millisecond)

	// Fill queue and worker using SubmitBlocking
	submitCtx := context.Background()
	for i := 0; i < 2; i++ {
		err = pool.SubmitBlocking(submitCtx, Task(func(ctx context.Context) {
			time.Sleep(300 * time.Millisecond)
		}, "jobName", fmt.Sprintf("fill-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// SubmitWithRetry should eventually succeed
	ctx := context.Background()
	var executed atomic.Bool
	err = pool.SubmitWithRetry(ctx, Task(func(ctx context.Context) {
		executed.Store(true)
	}, "jobName", "retry-task"), 10, 100*time.Millisecond)

	if err != nil {
		t.Errorf("SubmitWithRetry failed: %v", err)
	}

	// Wait for execution
	time.Sleep(1 * time.Second)

	if !executed.Load() {
		t.Error("retry task was not executed")
	}
}

// TestWorkerPool_GracefulShutdown tests graceful shutdown
func TestWorkerPool_GracefulShutdown(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "graceful-shutdown-test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	var completed atomic.Int32

	// Submit several tasks
	for i := 0; i < 5; i++ {
		err = pool.Submit(Task(func(ctx context.Context) {
			time.Sleep(200 * time.Millisecond)
			completed.Add(1)
		}, "jobName", fmt.Sprintf("shutdown-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	// Graceful shutdown should wait for tasks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pool.ShutdownGraceful(ctx)
	if err != nil {
		t.Errorf("graceful shutdown failed: %v", err)
	}

	// All tasks should have completed
	if count := completed.Load(); count != 5 {
		t.Errorf("expected 5 tasks completed, got %d", count)
	}
}

// TestWorkerPool_ImmediateShutdown tests immediate shutdown
func TestWorkerPool_ImmediateShutdown(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "immediate-shutdown-test",
		Workers:     1,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	var completed atomic.Int32

	// Submit several long tasks
	for i := 0; i < 5; i++ {
		err = pool.Submit(Task(func(ctx context.Context) {
			select {
			case <-time.After(5 * time.Second):
				completed.Add(1)
			case <-ctx.Done():
				return
			}
		}, "jobName", fmt.Sprintf("long-task-%d", i)))

		if err != nil {
			t.Errorf("failed to submit task %d: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Immediate shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = pool.Shutdown(ctx)
	if err != nil {
		t.Errorf("immediate shutdown failed: %v", err)
	}

	// Most tasks should not have completed
	if count := completed.Load(); count >= 3 {
		t.Errorf("expected fewer than 3 tasks completed, got %d", count)
	}
}

// TestWorkerPool_ConcurrentSubmissions tests thread safety
func TestWorkerPool_ConcurrentSubmissions(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "concurrent-test",
		Workers:     10,
		QueueSize:   150, // Large enough for all 100 tasks
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	var completed atomic.Int32
	var submitWg sync.WaitGroup
	var taskWg sync.WaitGroup

	// Spawn 10 goroutines each submitting 10 tasks
	submitCtx := context.Background()
	for i := 0; i < 10; i++ {
		submitWg.Add(1)
		go func() {
			defer submitWg.Done()
			for j := 0; j < 10; j++ {
				taskWg.Add(1)
				err := pool.SubmitBlocking(submitCtx, Task(func(ctx context.Context) {
					completed.Add(1)
					taskWg.Done()
				}, "jobName", "concurrent-task"))

				if err != nil {
					t.Errorf("failed to submit task: %v", err)
					taskWg.Done()
				}
			}
		}()
	}

	// Wait for all submissions
	submitWg.Wait()

	// Wait for all tasks to complete
	taskWg.Wait()

	if count := completed.Load(); count != 100 {
		t.Errorf("expected 100 tasks completed, got %d", count)
	}
}

// TestWorkerPool_Stats tests metrics tracking
func TestWorkerPool_Stats(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "stats-test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Submit successful task
	pool.Submit(Task(func(ctx context.Context) {
		time.Sleep(50 * time.Millisecond)
	}, "jobName", "success-task"))

	// Submit task that panics
	pool.Submit(Task(func(ctx context.Context) {
		panic("test panic")
	}, "jobName", "panic-task"))

	// Submit task that times out
	pool.Submit(Task(func(ctx context.Context) {
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return
		}
	}, "jobName", "timeout-task"))

	// Wait for tasks to complete
	time.Sleep(1 * time.Second)

	stats := pool.Stats()

	if stats["name"] != "stats-test" {
		t.Errorf("expected name 'stats-test', got %v", stats["name"])
	}

	if stats["workers"] != 2 {
		t.Errorf("expected 2 workers, got %v", stats["workers"])
	}

	completed := stats["tasks_completed"].(int64)
	if completed != 1 {
		t.Errorf("expected 1 completed task, got %d", completed)
	}

	panicked := stats["tasks_panicked"].(int64)
	if panicked != 1 {
		t.Errorf("expected 1 panicked task, got %d", panicked)
	}

	timedOut := stats["tasks_timed_out"].(int64)
	if timedOut != 1 {
		t.Errorf("expected 1 timed out task, got %d", timedOut)
	}
}

// TestWorkerPool_IsHealthy tests health check
func TestWorkerPool_IsHealthy(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "health-test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !pool.IsHealthy() {
		t.Error("pool should be healthy after start")
	}

	pool.ShutdownGraceful(context.Background())

	if pool.IsHealthy() {
		t.Error("pool should not be healthy after shutdown")
	}
}

// TestWorkerPool_ResetStats tests stats reset
func TestWorkerPool_ResetStats(t *testing.T) {
	pool, err := NewPoolAndStart(PoolConfig{
		Name:        "reset-stats-test",
		Workers:     2,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.ShutdownGraceful(context.Background())

	// Submit some tasks
	for i := 0; i < 5; i++ {
		pool.Submit(Task(func(ctx context.Context) {
			time.Sleep(50 * time.Millisecond)
		}, "jobName", "test-task"))
	}

	time.Sleep(500 * time.Millisecond)

	stats := pool.Stats()
	if stats["tasks_completed"].(int64) != 5 {
		t.Errorf("expected 5 completed tasks before reset, got %d", stats["tasks_completed"])
	}

	// Reset stats
	pool.ResetStats()

	stats = pool.Stats()
	if stats["tasks_completed"].(int64) != 0 {
		t.Errorf("expected 0 completed tasks after reset, got %d", stats["tasks_completed"])
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
