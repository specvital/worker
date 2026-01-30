package reliability

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64 // Backoff multiplier (default: 2.0)
	JitterFactor   float64 // Random jitter factor 0-1 (default: 0.1)
}

// DefaultPhase1RetryConfig returns default retry config for Phase 1.
func DefaultPhase1RetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 2 * time.Second,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		JitterFactor:   0.1,
	}
}

// DefaultPhase2RetryConfig returns default retry config for Phase 2.
func DefaultPhase2RetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    2,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		JitterFactor:   0.1,
	}
}

// RetryableError indicates an error that should trigger retry.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error should trigger a retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for RetryableError wrapper
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return true
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Circuit breaker open is not retryable
	if errors.Is(err, ErrCircuitOpen) {
		return false
	}

	// Check error message for common retryable patterns
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"rate limit",
		"quota exceeded",
		"too many requests",
		"service unavailable",
		"internal server error",
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// IsRetryableStatusCode checks if an HTTP status code indicates retryable error.
func IsRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusBadGateway,
		http.StatusInternalServerError:
		return true
	default:
		return false
	}
}

// Retryer performs operations with retry logic.
type Retryer struct {
	config RetryConfig
}

// NewRetryer creates a new retryer with the given configuration.
func NewRetryer(config RetryConfig) *Retryer {
	if config.Multiplier == 0 {
		config.Multiplier = 2.0
	}
	if config.JitterFactor == 0 {
		config.JitterFactor = 0.1
	}
	return &Retryer{config: config}
}

// Do executes the operation with retry logic.
func (r *Retryer) Do(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if not retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't retry on last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate backoff with jitter
		backoff := r.calculateBackoff(attempt)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateBackoff calculates the backoff duration with exponential increase and jitter.
func (r *Retryer) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: initial * multiplier^(attempt-1)
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Cap at max backoff
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	// Add jitter: backoff * (1 + random(-jitter, +jitter))
	jitter := (rand.Float64()*2 - 1) * r.config.JitterFactor
	backoff *= (1 + jitter)

	return time.Duration(backoff)
}
