package workerpool

// ErrorNotifier defines the interface for external error notification services.
// Implement this interface to integrate with error tracking services like
// Bugsnag, Sentry, Rollbar, etc.
//
// Example implementation:
//
//	type MyNotifier struct {
//	    client *sentry.Client
//	}
//
//	func (n *MyNotifier) NotifyError(err error, metadata map[string]any) {
//	    n.client.CaptureException(err, metadata)
//	}
//
//	func (n *MyNotifier) NotifyPanic(recovered any, metadata map[string]any) {
//	    n.client.CaptureException(fmt.Errorf("panic: %v", recovered), metadata)
//	}
type ErrorNotifier interface {
	// NotifyError sends an error notification with optional metadata
	NotifyError(err error, metadata map[string]any)

	// NotifyWarning sends a warning notification with optional metadata
	NotifyWarning(err error, metadata map[string]any)

	// NotifyPanic sends a panic notification with optional metadata
	// The recovered parameter contains the value from recover()
	NotifyPanic(recovered any, metadata map[string]any)
}
