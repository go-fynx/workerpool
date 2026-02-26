package workerpool

import (
	"fmt"

	"github.com/bugsnag/bugsnag-go/v2"
)

// BugsnagNotifier implements ErrorNotifier interface for Bugsnag integration.
// This allows optional Bugsnag error reporting without forcing all users to use it.
//
// Usage:
//
//	// Initialize Bugsnag first
//	bugsnag.Configure(bugsnag.Configuration{
//	    APIKey: "your-api-key",
//	    AppVersion: "1.0.0",
//	})
//
//	// Create notifier and attach to pool
//	notifier := workerpool.NewBugsnagNotifier()
//	pool, _ := workerpool.NewWorkerPool(workerpool.PoolConfig{
//	    Name: "my-pool",
//	    Workers: 10,
//	    QueueSize: 100,
//	    ErrorNotifier: notifier,
//	})
type BugsnagNotifier struct{}

// NewBugsnagNotifier creates a new Bugsnag error notifier.
// Make sure to configure Bugsnag before using this notifier.
func NewBugsnagNotifier() *BugsnagNotifier {
	return &BugsnagNotifier{}
}

// NotifyError sends an error-level notification to Bugsnag.
func (b *BugsnagNotifier) NotifyError(err error, metadata map[string]any) {
	bugsnag.Notify(err, bugsnag.MetaData{"meta_data": metadata}, bugsnag.SeverityError)
}

// NotifyWarning sends a warning-level notification to Bugsnag.
func (b *BugsnagNotifier) NotifyWarning(err error, metadata map[string]any) {
	bugsnag.Notify(err, bugsnag.MetaData{"meta_data": metadata}, bugsnag.SeverityWarning)
}

// NotifyPanic sends a panic notification to Bugsnag with unhandled state.
func (b *BugsnagNotifier) NotifyPanic(recovered any, metadata map[string]any) {
	state := bugsnag.HandledState{
		SeverityReason:   bugsnag.SeverityReasonUnhandledMiddlewareError,
		OriginalSeverity: bugsnag.SeverityError,
		Unhandled:        true,
	}
	bugsnag.Notify(fmt.Errorf("panic: %v", recovered), state, bugsnag.MetaData{"meta_data": metadata})
}
