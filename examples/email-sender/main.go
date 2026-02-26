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

// Email represents an email to send
type Email struct {
	To      string
	Subject string
	Body    string
}

func main() {
	fmt.Println("=== Email Sender Example ===")

	// Create pool for sending emails
	pool, err := workerpool.NewPoolAndStart(workerpool.PoolConfig{
		Name:        "email-sender",
		Workers:     10,
		QueueSize:   1000,
		TaskTimeout: 30 * time.Second,
		Logger:      workerpool.NewDefaultLogger(workerpool.LogLevelInfo),
	})
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		pool.ShutdownGraceful(ctx)
	}()

	// Track email statistics
	var sent atomic.Int64
	var failed atomic.Int64
	var bounced atomic.Int64

	// Generate test emails
	emails := generateEmails(100)
	
	fmt.Printf("\nSending %d emails...\n", len(emails))
	start := time.Now()

	// Submit emails with retry on failure
	ctx := context.Background()
	for _, email := range emails {
		em := email

		err := pool.SubmitWithRetry(ctx, workerpool.Task(func(ctx context.Context) {
			err := sendEmail(ctx, em)
			if err != nil {
				if err.Error() == "email bounced" {
					bounced.Add(1)
					log.Printf("Email to %s bounced", em.To)
				} else {
					failed.Add(1)
					log.Printf("Failed to send email to %s: %v", em.To, err)
				}
			} else {
				sent.Add(1)
			}
		}, "jobName", "send-email", "recipient", em.To), 3, 500*time.Millisecond)

		if err != nil {
			log.Printf("Failed to queue email to %s: %v", em.To, err)
			failed.Add(1)
		}
	}

	fmt.Println("\nAll emails queued, waiting for delivery...")

	// Wait for completion
	pool.ShutdownGraceful(context.Background())

	elapsed := time.Since(start)

	// Print results
	fmt.Printf("\n=== Email Results ===\n")
	fmt.Printf("Total Emails: %d\n", len(emails))
	fmt.Printf("Successfully Sent: %d\n", sent.Load())
	fmt.Printf("Bounced: %d\n", bounced.Load())
	fmt.Printf("Failed: %d\n", failed.Load())
	fmt.Printf("Time Elapsed: %v\n", elapsed)
	fmt.Printf("Throughput: %.2f emails/second\n", float64(len(emails))/elapsed.Seconds())

	stats := pool.Stats()
	fmt.Printf("\n=== Pool Statistics ===\n")
	fmt.Printf("Tasks Completed: %d\n", stats["tasks_completed"])
	fmt.Printf("Tasks Timed Out: %d\n", stats["tasks_timed_out"])
	fmt.Printf("Tasks Panicked: %d\n", stats["tasks_panicked"])
}

func generateEmails(count int) []Email {
	emails := make([]Email, count)
	for i := 0; i < count; i++ {
		emails[i] = Email{
			To:      fmt.Sprintf("user%d@example.com", i+1),
			Subject: fmt.Sprintf("Test Email %d", i+1),
			Body:    fmt.Sprintf("This is test email number %d", i+1),
		}
	}
	return emails
}

func sendEmail(ctx context.Context, email Email) error {
	// Simulate email sending time (100-500ms)
	sendTime := time.Duration(100+rand.Intn(400)) * time.Millisecond

	select {
	case <-time.After(sendTime):
		// Simulate occasional bounces (3% rate)
		if rand.Intn(100) < 3 {
			return fmt.Errorf("email bounced")
		}
		// Simulate occasional failures (2% rate)
		if rand.Intn(100) < 2 {
			return fmt.Errorf("SMTP error: connection refused")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
