package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-fynx/workerpool"
)

func main() {
	fmt.Println("=== Basic WorkerPool Example ===")

	// Create and start a worker pool
	pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
		Name:        "basic-example",
		Workers:     3,
		QueueSize:   10,
		TaskTimeout: 5 * time.Second,
		Logger:      workerpool.NewDefaultLogger(workerpool.LogLevelInfo),
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}

	// Ensure graceful shutdown
	defer func() {
		fmt.Println("\nShutting down pool...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := pool.ShutdownGraceful(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		fmt.Println("Pool shutdown complete")
	}()

	// Submit some tasks
	fmt.Println("\nSubmitting tasks...")
	for i := 1; i <= 10; i++ {
		taskID := i
		err := pool.Submit(workerpool.Task(func(ctx context.Context) {
			fmt.Printf("Task %d started\n", taskID)
			
			// Simulate some work
			select {
			case <-time.After(time.Duration(taskID*100) * time.Millisecond):
				fmt.Printf("Task %d completed\n", taskID)
			case <-ctx.Done():
				fmt.Printf("Task %d cancelled\n", taskID)
				return
			}
		}, "jobName", "basic-task", "taskID", taskID))

		if err != nil {
			log.Printf("Failed to submit task %d: %v", taskID, err)
		}
	}

	// Wait a bit to see tasks execute
	time.Sleep(3 * time.Second)

	// Print statistics
	fmt.Println("\n=== Pool Statistics ===")
	stats := pool.Stats()
	fmt.Printf("Pool Name: %s\n", stats["name"])
	fmt.Printf("Workers: %d\n", stats["workers"])
	fmt.Printf("Queue Depth: %d / %d\n", stats["queue_depth"], stats["queue_capacity"])
	fmt.Printf("Queue Utilization: %s\n", stats["queue_utilization"])
	fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
	fmt.Printf("Tasks Timed Out: %d\n", stats["tasks_timed_out"])
	fmt.Printf("Tasks Panicked: %d\n", stats["tasks_panicked"])
}
