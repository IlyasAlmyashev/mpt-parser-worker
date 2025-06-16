package scrapers

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrNoMoreProducts signals that no products were found on a page and we can stop.
var ErrNoMoreProducts = errors.New("no more products")

// Retry calls fn up to cfg.Attempts times with exponential backoff.
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	backoff := cfg.InitialBackoff
	for attempt := 1; attempt <= cfg.Attempts; attempt++ {
		if err := fn(); err != nil {
			if attempt == cfg.Attempts {
				return err
			}
			select {
			case <-time.After(backoff):
				backoff *= 2
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
	return nil
}

// StartWorkerPool runs the given workerCount goroutines which process tasks from tasksCh.
// The worker function accepts a context and a task.
// If ErrNoMoreProducts is returned, the pool will break early.
func StartWorkerPool[T any](
	ctx context.Context,
	tasks <-chan T,
	workerCount int,
	fn func(ctx context.Context, task T) error,
) error {
	var wg sync.WaitGroup
	errCh := make(chan error, workerCount)

	// A guard to signal when we should stop reading tasks.
	doneCh := make(chan struct{})

	// Start workers
	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case <-doneCh:
					return
				case task, ok := <-tasks:
					if !ok {
						return
					}
					// Execute a worker function.
					err := fn(ctx, task)
					if err != nil {
						// If it's ErrNoMoreProducts, signal done to all workers but keep track of it.
						if errors.Is(err, ErrNoMoreProducts) {
							select {
							case errCh <- err: // notify manager
							default:
							}
							// Signal an immediate stop to all goroutines.
							close(doneCh)
							return
						}
						// If we have any other error, send it to channel (non-blocking).
						select {
						case errCh <- err:
						default:
						}
					}
				}
			}
		}()
	}

	// Wait for workers to finish.
	wg.Wait()
	close(errCh)

	// Return the first error, if any.
	for e := range errCh {
		return e
	}
	return nil
}
