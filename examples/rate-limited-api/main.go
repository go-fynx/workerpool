package main

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/go-fynx/workerpool"
)

func main() {
	fmt.Println("=== Rate-Limited API Client Example ===")

	// Create pool for rate-limited API calls
	// API allows 50 requests per second, so we use 50 workers
	pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
		Name:        "api-client",
		Workers:     50,
		QueueSize:   500,
		TaskTimeout: 15 * time.Second,
		Logger:      workerpool.NewDefaultLogger(workerpool.LogLevelInfo),
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.ShutdownGraceful(ctx)
	}()

	// Track API call statistics
	var successful atomic.Int64
	var failed atomic.Int64
	var rateLimited atomic.Int64

	// Simulate 500 API calls
	const totalCalls = 500
	fmt.Printf("\nMaking %d API calls with rate limiting...\n", totalCalls)

	start := time.Now()

	for i := 1; i <= totalCalls; i++ {
		callID := i

		err := pool.Submit(workerpool.Task(func(ctx context.Context) {
			// Make API call
			err := makeAPICall(ctx, callID)
			if err != nil {
				if err.Error() == "rate limited" {
					rateLimited.Add(1)
					log.Printf("API call %d: rate limited", callID)
				} else {
					failed.Add(1)
					log.Printf("API call %d: failed - %v", callID, err)
				}
			} else {
				successful.Add(1)
			}
		}, "jobName", "api-call", "callID", callID))

		if err != nil {
			log.Printf("Failed to submit API call %d: %v", callID, err)
		}

		// Add small delay to spread out submissions
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println("\nAll API calls submitted, waiting for completion...")

	// Wait for completion
	pool.ShutdownGraceful(context.Background())

	elapsed := time.Since(start)

	// Print results
	fmt.Printf("\n=== API Call Results ===\n")
	fmt.Printf("Total Calls: %d\n", totalCalls)
	fmt.Printf("Successful: %d\n", successful.Load())
	fmt.Printf("Failed: %d\n", failed.Load())
	fmt.Printf("Rate Limited: %d\n", rateLimited.Load())
	fmt.Printf("Time Elapsed: %v\n", elapsed)
	fmt.Printf("Actual Rate: %.2f calls/second\n", float64(totalCalls)/elapsed.Seconds())

	stats := pool.Stats()
	fmt.Printf("\n=== Pool Statistics ===\n")
	fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
	fmt.Printf("Tasks Timed Out: %d\n", stats["tasks_timed_out"])
}

func makeAPICall(ctx context.Context, callID int) error {
	// Simulate API call duration (50-200ms)
	callDuration := time.Duration(50+callID%150) * time.Millisecond

	select {
	case <-time.After(callDuration):
		// Simulate occasional failures (5% rate)
		if callID%20 == 0 {
			return fmt.Errorf("API error: service unavailable")
		}
		// Simulate rate limiting (2% rate)
		if callID%50 == 0 {
			return fmt.Errorf("rate limited")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
