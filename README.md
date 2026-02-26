# WorkerPool


<div align="center">
**A production-ready Go worker pool library for managing concurrent task execution with controlled resource usage, timeout enforcement, panic recovery, and comprehensive metrics.**

![workerpool](workerpool.png)


[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue)](https://golang.org/dl/)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-fynx/workerpool.svg)](https://pkg.go.dev/github.com/go-fynx/workerpool)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-fynx/workerpool)](https://goreportcard.com/report/github.com/go-fynx/workerpool)

</div>


## Features

- **Fixed Worker Pool**: Control concurrency with a fixed number of worker goroutines
- **Buffered Task Queue**: Handle traffic bursts with configurable queue size
- **Automatic Timeout**: Per-task timeout enforcement with context cancellation
- **Panic Recovery**: Automatic panic recovery with detailed error reporting
- **Graceful Shutdown**: Two shutdown modes - graceful (wait for tasks) and immediate
- **Comprehensive Metrics**: Track completed, timed-out, and panicked tasks
- **Thread-Safe**: All operations are safe for concurrent use
- **Flexible Logging**: Pluggable logger interface (works with any logger)
- **Error Tracking**: Optional integration with error services (Bugsnag, Sentry, etc.)
- **Retry Support**: Built-in retry mechanism with configurable backoff
- **Health Checks**: Monitor pool health and queue utilization

## Installation

```bash
go get github.com/go-fynx/workerpool
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/go-fynx/workerpool"
)

func main() {
    // Create and start a worker pool
    pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
        Name:        "email-sender",
        Workers:     10,
        QueueSize:   100,
        TaskTimeout: 30 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer pool.ShutdownGraceful(context.Background())

    // Submit tasks
    for i := 0; i < 50; i++ {
        userID := i
        err := pool.Submit(workerpool.Task(func(ctx context.Context) {
            // Your work here
            sendEmail(ctx, userID)
        }, "jobName", "send-welcome-email", "userID", userID))
        
        if err != nil {
            log.Printf("Failed to submit task: %v", err)
        }
    }

    log.Println("All tasks submitted")
}

func sendEmail(ctx context.Context, userID int) {
    // Respect context cancellation
    select {
    case <-ctx.Done():
        return
    default:
        // Send email logic
        log.Printf("Sending email to user %d", userID)
    }
}
```

## Configuration

### PoolConfig Options

```go
type PoolConfig struct {
    Name          string              // Pool identifier for logs/metrics
    Workers       int                 // Number of concurrent workers (1-1000)
    QueueSize     int                 // Task queue buffer size (1-10000)
    TaskTimeout   time.Duration       // Max duration per task (default: 30s)
    Logger        Logger              // Custom logger (default: stdout logger)
    ErrorNotifier ErrorNotifier       // Optional error tracking integration
}
```

### Default Configuration

```go
config := workerpool.DefaultPoolConfig("my-pool")
// Returns:
// - Workers: 5
// - QueueSize: 50 (10x workers)
// - TaskTimeout: 30 seconds
// - Logger: DefaultLogger with Info level
```

### Builder Pattern

```go
config := workerpool.DefaultPoolConfig("api-handlers").
    SetWorkers(20).
    SetQueueSize(200).
    SetTaskTimeout(10 * time.Second).
    SetLogger(customLogger).
    SetErrorNotifier(bugsnagNotifier)

pool, err := workerpool.NewWorkerPool(config)
```

## Usage Examples

### Basic Task Submission

```go
err := pool.Submit(workerpool.Task(func(ctx context.Context) {
    // Your task logic
    processData(ctx)
}, "jobName", "process-data", "batchID", 123))

if err != nil {
    // Queue is full or pool is shutting down
    log.Printf("Submit failed: %v", err)
}
```

### Blocking Submission (Wait for Space)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := pool.SubmitBlocking(ctx, workerpool.Task(func(ctx context.Context) {
    processData(ctx)
}, "jobName", "process-data"))
```

### Submission with Retry

```go
ctx := context.Background()
err := pool.SubmitWithRetry(
    ctx,
    workerpool.Task(func(ctx context.Context) {
        processData(ctx)
    }, "jobName", "process-data"),
    5,                      // max retries
    100*time.Millisecond,   // backoff duration
)
```

### Context-Aware Tasks

Always respect context cancellation for timeouts and shutdowns:

```go
pool.Submit(workerpool.Task(func(ctx context.Context) {
    for i := 0; i < 100; i++ {
        select {
        case <-ctx.Done():
            log.Println("Task cancelled")
            return
        default:
            processItem(i)
        }
    }
}, "jobName", "batch-process"))
```

### Graceful Shutdown

```go
// Wait for all queued tasks to complete (with timeout)
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := pool.ShutdownGraceful(ctx); err != nil {
    log.Printf("Graceful shutdown timeout: %v", err)
}
```

### Immediate Shutdown

```go
// Stop immediately, abandon queued tasks
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := pool.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

## Monitoring and Metrics

### Get Pool Statistics

```go
stats := pool.Stats()
fmt.Printf("Pool: %s\n", stats["name"])
fmt.Printf("Workers: %d\n", stats["workers"])
fmt.Printf("Queue Depth: %d / %d\n", stats["queue_depth"], stats["queue_capacity"])
fmt.Printf("Queue Utilization: %s\n", stats["queue_utilization"])
fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
fmt.Printf("Tasks Timed Out: %d\n", stats["tasks_timed_out"])
fmt.Printf("Tasks Panicked: %d\n", stats["tasks_panicked"])
```

### Health Check

```go
if pool.IsHealthy() {
    fmt.Println("Pool is running and accepting tasks")
} else {
    fmt.Println("Pool is shutting down or not started")
}
```

### Reset Statistics

```go
pool.ResetStats() // Reset counters to zero
```

## Custom Logger

Implement the `Logger` interface to use your own logger:

```go
type Logger interface {
    Debug(args ...any)
    Debugf(format string, args ...any)
    Info(args ...any)
    Infof(format string, args ...any)
    Warn(args ...any)
    Warnf(format string, args ...any)
    Error(args ...any)
    Errorf(format string, args ...any)
}
```

### Example with Zap

```go
import "go.uber.org/zap"

type ZapLogger struct {
    logger *zap.SugaredLogger
}

func (z *ZapLogger) Info(args ...any) {
    z.logger.Info(args...)
}
// ... implement other methods

pool, _ := workerpool.NewWorkerPool(workerpool.PoolConfig{
    Logger: &ZapLogger{logger: zapLogger.Sugar()},
})
```

### Disable Logging

```go
pool, _ := workerpool.NewWorkerPool(workerpool.PoolConfig{
    Logger: workerpool.NewNoOpLogger(),
})
```

## Error Tracking Integration

### Bugsnag Example

```go
import "github.com/bugsnag/bugsnag-go/v2"

// Configure Bugsnag
bugsnag.Configure(bugsnag.Configuration{
    APIKey:     "your-api-key",
    AppVersion: "1.0.0",
})

// Create pool with Bugsnag notifier
pool, _ := workerpool.NewWorkerPool(workerpool.PoolConfig{
    Name:          "my-pool",
    Workers:       10,
    QueueSize:     100,
    ErrorNotifier: workerpool.NewBugsnagNotifier(),
})
```

### Custom Error Notifier

```go
type ErrorNotifier interface {
    NotifyError(err error, metadata map[string]any)
    NotifyWarning(err error, metadata map[string]any)
    NotifyPanic(recovered any, metadata map[string]any)
}

// Example: Sentry integration
type SentryNotifier struct {
    hub *sentry.Hub
}

func (s *SentryNotifier) NotifyPanic(recovered any, metadata map[string]any) {
    s.hub.CaptureException(fmt.Errorf("panic: %v", recovered))
}
// ... implement other methods
```

## Best Practices

### 1. Choose Appropriate Pool Size

```go
// CPU-bound tasks: Use runtime.NumCPU()
workers := runtime.NumCPU()

// I/O-bound tasks: Use higher values (10-100+)
workers := 50

// Database operations: Match connection pool size
workers := 20
```

### 2. Set Reasonable Timeouts

```go
// Quick API calls
TaskTimeout: 5 * time.Second

// Database queries
TaskTimeout: 30 * time.Second

// File processing
TaskTimeout: 2 * time.Minute
```

### 3. Queue Size Guidelines

```go
// Typical: 5-10x the number of workers
QueueSize: workers * 10

// Low memory: Smaller queue
QueueSize: workers * 2

// Burst handling: Larger queue
QueueSize: workers * 50
```

### 4. Always Defer Shutdown

```go
pool, err := workerpool.NewPoolAndStart(config)
if err != nil {
    return err
}
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    pool.ShutdownGraceful(ctx)
}()
```

### 5. Handle Submit Errors

```go
err := pool.Submit(task)
if err != nil {
    // Queue is full - implement backpressure
    if strings.Contains(err.Error(), "queue is full") {
        // Option 1: Use SubmitWithRetry
        // Option 2: Use SubmitBlocking
        // Option 3: Drop task and log
    }
}
```

### 6. Monitor Queue Utilization

```go
stats := pool.Stats()
utilization := stats["queue_depth"].(int) * 100 / stats["queue_capacity"].(int)

if utilization > 80 {
    log.Warn("Queue utilization high, consider scaling")
}
```

## Use Cases

### High-Volume API Operations

```go
pool, _ := workerpool.NewPoolAndStart(workerpool.PoolConfig{
    Name:        "api-caller",
    Workers:     50,
    QueueSize:   500,
    TaskTimeout: 10 * time.Second,
})
```

### Database Batch Operations

```go
pool, _ := workerpool.NewPoolAndStart(workerpool.PoolConfig{
    Name:        "db-writer",
    Workers:     20,  // Match DB connection pool
    QueueSize:   200,
    TaskTimeout: 30 * time.Second,
})
```

### Email/Notification Sending

```go
pool, _ := workerpool.NewPoolAndStart(workerpool.PoolConfig{
    Name:        "email-sender",
    Workers:     10,
    QueueSize:   1000,
    TaskTimeout: 15 * time.Second,
})
```

### Image/File Processing

```go
pool, _ := workerpool.NewPoolAndStart(workerpool.PoolConfig{
    Name:        "image-processor",
    Workers:     runtime.NumCPU(),
    QueueSize:   100,
    TaskTimeout: 2 * time.Minute,
})
```

## Error Handling

### Task Panics

Panics are automatically recovered and logged. The pool continues operating:

```go
pool.Submit(workerpool.Task(func(ctx context.Context) {
    panic("oops") // Pool will recover, log, and continue
}, "jobName", "risky-task"))

// Check panic count
stats := pool.Stats()
fmt.Printf("Panicked tasks: %d\n", stats["tasks_panicked"])
```

### Task Timeouts

Tasks that exceed timeout are cancelled via context:

```go
pool.Submit(workerpool.Task(func(ctx context.Context) {
    select {
    case <-time.After(60 * time.Second):
        // This won't complete if timeout is 30s
    case <-ctx.Done():
        log.Println("Task timed out")
        return
    }
}, "jobName", "long-task"))
```

## Performance Considerations

- **Memory**: Each worker is a goroutine (~2KB). Queue buffer holds task pointers
- **Throughput**: Scales with worker count and task duration
- **Latency**: Queue depth + worker availability affects task start time
- **CPU**: Worker count should not exceed available cores for CPU-bound tasks

## Testing

Run tests:

```bash
go test -v
go test -race -v  # Check for race conditions
```

Run benchmarks:

```bash
go test -bench=. -benchmem
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Support

- **Issues**: [GitHub Issues](https://github.com/go-fynx/workerpool/issues)
- **Documentation**: See code documentation for detailed API reference

## Roadmap

- [ ] Dynamic worker scaling
- [ ] Task priorities
- [ ] Prometheus metrics exporter
- [ ] OpenTelemetry tracing support
- [ ] Circuit breaker integration
- [ ] Rate limiting

---