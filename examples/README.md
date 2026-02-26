# WorkerPool Examples

This directory contains practical examples demonstrating different use cases for the WorkerPool library.

## Running Examples

Each example is a standalone program. To run an example:

```bash
# Replace yourusername with your actual GitHub username in go.mod first
cd examples/basic
go mod init example
go mod edit -replace github.com/go-fynx/workerpool=../..
go mod tidy
go run main.go
```

## Examples

### 1. Basic (`basic/`)

Demonstrates fundamental WorkerPool usage:
- Creating and starting a pool
- Submitting simple tasks
- Graceful shutdown
- Viewing pool statistics

**Best for**: Learning the basics

```bash
cd examples/basic
go run main.go
```

### 2. Batch Processing (`batch-processing/`)

Shows how to process large batches of data efficiently:
- Processing 1000 records concurrently
- Tracking success/failure rates
- Measuring throughput
- Using `SubmitBlocking` for backpressure

**Best for**: Database operations, data transformations, bulk processing

**Features**:
- 20 workers for parallel processing
- 200-item queue buffer
- Progress tracking with atomic counters
- Throughput metrics

```bash
cd examples/batch-processing
go run main.go
```

### 3. Rate-Limited API Client (`rate-limited-api/`)

Demonstrates calling external APIs with rate limiting:
- Respecting API rate limits (50 requests/second)
- Handling API failures and retries
- Tracking rate limit errors
- Spreading request load

**Best for**: External API integrations, webhooks, microservice calls

**Features**:
- 50 workers matching API rate limit
- 500-item queue for burst handling
- Error categorization (failed vs rate-limited)
- Actual throughput measurement

```bash
cd examples/rate-limited-api
go run main.go
```

### 4. Email Sender (`email-sender/`)

Shows bulk email sending with retry logic:
- Sending 100 emails concurrently
- Retry on transient failures (3 retries, 500ms backoff)
- Tracking bounces vs failures
- Queue-based delivery

**Best for**: Email campaigns, notifications, messaging systems

**Features**:
- 10 workers for email delivery
- 1000-item queue for large batches
- Automatic retry with `SubmitWithRetry`
- Separate tracking of bounces and failures
- Delivery statistics

```bash
cd examples/email-sender
go run main.go
```

## Common Patterns

### Pattern 1: Context-Aware Tasks

All examples show proper context handling:

```go
pool.Submit(workerpool.Task(func(ctx context.Context) {
    select {
    case <-time.After(processTime):
        // Work completed
    case <-ctx.Done():
        // Cancelled or timed out
        return
    }
}, "jobName", "task-name"))
```

### Pattern 2: Error Handling

Track different error types:

```go
var successful, failed, rateLimited atomic.Int64

pool.Submit(workerpool.Task(func(ctx context.Context) {
    if err := doWork(ctx); err != nil {
        if err == ErrRateLimited {
            rateLimited.Add(1)
        } else {
            failed.Add(1)
        }
    } else {
        successful.Add(1)
    }
}))
```

### Pattern 3: Graceful Shutdown with Timeout

Always use deferred shutdown:

```go
defer func() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    if err := pool.ShutdownGraceful(ctx); err != nil {
        log.Printf("Shutdown timeout: %v", err)
    }
}()
```

### Pattern 4: Progress Tracking

Use atomic counters for safe concurrent updates:

```go
var processed atomic.Int64

for _, item := range items {
    pool.Submit(workerpool.Task(func(ctx context.Context) {
        processItem(item)
        processed.Add(1)
    }))
}
```

## Performance Tips

1. **Worker Count**:
   - CPU-bound: `runtime.NumCPU()`
   - I/O-bound: 10-100 (depends on latency)
   - Rate-limited: Match the rate limit

2. **Queue Size**:
   - Typical: 5-10x worker count
   - Burst handling: 20-50x worker count
   - Low memory: 2-3x worker count

3. **Task Timeout**:
   - Fast operations: 1-5 seconds
   - Network calls: 10-30 seconds
   - Heavy processing: 1-5 minutes

## Customization

Each example can be customized by modifying the `PoolConfig`:

```go
pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
    Name:        "custom-pool",
    Workers:     20,              // Adjust for your workload
    QueueSize:   200,             // Adjust for expected burst
    TaskTimeout: 30 * time.Second, // Adjust for task duration
    Logger:      customLogger,     // Use your logger
    ErrorNotifier: notifier,       // Optional error tracking
})
```

## Troubleshooting

### Queue Full Errors

If you see "queue is full" errors:
1. Increase `QueueSize`
2. Use `SubmitBlocking` instead of `Submit`
3. Use `SubmitWithRetry` with backoff
4. Add more workers to process faster

### Tasks Timing Out

If tasks frequently timeout:
1. Increase `TaskTimeout`
2. Optimize task execution
3. Check for blocking operations
4. Ensure tasks respect context cancellation

### Low Throughput

If throughput is lower than expected:
1. Increase worker count
2. Reduce task execution time
3. Check for contention (locks, shared resources)
4. Profile your task functions

## Next Steps

- Read the main [README.md](../README.md) for full API documentation
- Check [CHANGELOG.md](../CHANGELOG.md) for version history
- See the [test files](../workerpool_test.go) for more examples
