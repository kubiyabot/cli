package util

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RandomizeFactor float64
	RetryableFunc   func(error) bool
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        30 * time.Second,
		Multiplier:      2.0,
		RandomizeFactor: 0.3,
		RetryableFunc:   IsRetryableError,
	}
}

// IsRetryableError determines if an error is retryable
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for context errors (not retryable)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for specific error types
	var tempErr interface{ Temporary() bool }
	if errors.As(err, &tempErr) && tempErr.Temporary() {
		return true
	}

	// Check for timeout errors
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return true
	}

	// Default to retryable for unknown errors
	return true
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, config *RetryConfig, operation string, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableFunc(err) {
			return fmt.Errorf("%s failed (non-retryable): %w", operation, err)
		}

		// Check if we've exhausted attempts
		if attempt >= config.MaxAttempts {
			break
		}

		// Calculate next delay with jitter
		nextDelay := calculateDelay(delay, config.RandomizeFactor, config.MaxDelay)

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s cancelled after %d attempts: %w", operation, attempt, ctx.Err())
		case <-time.After(nextDelay):
			// Continue to next attempt
		}

		// Increase delay for next attempt
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("%s failed after %d attempts: %w", operation, config.MaxAttempts, lastErr)
}

// calculateDelay adds jitter to the delay
func calculateDelay(base time.Duration, randomizeFactor float64, maxDelay time.Duration) time.Duration {
	// Add randomization
	jitter := float64(base) * randomizeFactor
	minDelay := float64(base) - jitter
	maxJitteredDelay := float64(base) + jitter

	// Get random delay in range
	delay := minDelay + (rand.Float64() * (maxJitteredDelay - minDelay))

	// Ensure we don't exceed max delay
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	// Ensure minimum delay of 1ms
	if delay < 1 {
		delay = 1
	}

	return time.Duration(delay)
}

// ExponentialBackoff is a simple exponential backoff implementation
func ExponentialBackoff(attempt int, baseDelay time.Duration, maxDelay time.Duration) time.Duration {
	delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

// Retry is a simple retry wrapper
func Retry(attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("after %d attempts, last error: %w", attempts, err)
}

// RetryWithExponentialBackoff retries with exponential backoff
func RetryWithExponentialBackoff(ctx context.Context, attempts int, fn func() error) error {
	config := &RetryConfig{
		MaxAttempts:     attempts,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		Multiplier:      2.0,
		RandomizeFactor: 0.3,
		RetryableFunc:   IsRetryableError,
	}
	return RetryWithBackoff(ctx, config, "operation", fn)
}
