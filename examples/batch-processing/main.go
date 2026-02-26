package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/go-fynx/workerpool"
)

// DataRecord represents a data item to process
type DataRecord struct {
	ID      int
	Payload string
}

func main() {
	fmt.Println("=== Batch Processing Example ===")

	// Simulate a batch of 1000 records
	const batchSize = 1000
	records := generateRecords(batchSize)

	// Create pool optimized for batch processing
	pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
		Name:        "batch-processor",
		Workers:     20,
		QueueSize:   200,
		TaskTimeout: 10 * time.Second,
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

	// Track progress
	var processed atomic.Int64
	var failed atomic.Int64

	start := time.Now()
	fmt.Printf("\nProcessing %d records...\n", batchSize)

	// Submit all records for processing
	ctx := context.Background()
	for _, record := range records {
		rec := record
		
		err := pool.SubmitBlocking(ctx, workerpool.Task(func(ctx context.Context) {
			if err := processRecord(ctx, rec); err != nil {
				failed.Add(1)
				log.Printf("Failed to process record %d: %v", rec.ID, err)
			} else {
				processed.Add(1)
			}
		}, "jobName", "batch-process", "recordID", rec.ID))

		if err != nil {
			log.Printf("Failed to submit record %d: %v", rec.ID, err)
			failed.Add(1)
		}
	}

	fmt.Println("All records submitted, waiting for completion...")

	// Wait for all tasks to complete
	pool.ShutdownGraceful(context.Background())

	elapsed := time.Since(start)

	// Print results
	fmt.Printf("\n=== Processing Results ===\n")
	fmt.Printf("Total Records: %d\n", batchSize)
	fmt.Printf("Successfully Processed: %d\n", processed.Load())
	fmt.Printf("Failed: %d\n", failed.Load())
	fmt.Printf("Time Elapsed: %v\n", elapsed)
	fmt.Printf("Throughput: %.2f records/second\n", float64(batchSize)/elapsed.Seconds())

	stats := pool.Stats()
	fmt.Printf("\n=== Pool Statistics ===\n")
	fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
	fmt.Printf("Tasks Timed Out: %d\n", stats["tasks_timed_out"])
	fmt.Printf("Tasks Panicked: %d\n", stats["tasks_panicked"])
}

func generateRecords(count int) []DataRecord {
	records := make([]DataRecord, count)
	for i := 0; i < count; i++ {
		records[i] = DataRecord{
			ID:      i + 1,
			Payload: fmt.Sprintf("Data-%d", i+1),
		}
	}
	return records
}

func processRecord(ctx context.Context, record DataRecord) error {
	// Simulate processing time (10-50ms)
	processingTime := time.Duration(10+rand.Intn(40)) * time.Millisecond

	select {
	case <-time.After(processingTime):
		// Simulate 1% failure rate
		if rand.Intn(100) == 0 {
			return fmt.Errorf("random processing error")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
