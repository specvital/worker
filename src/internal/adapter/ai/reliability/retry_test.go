package reliability

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetryer_SucceedsOnFirstAttempt(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
		JitterFactor:   0.0,
	}
	r := NewRetryer(config)

	attempts := 0
	err := r.Do(context.Background(), func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got: %d", attempts)
	}
}

func TestRetryer_RetriesOnRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
		JitterFactor:   0.0,
	}
	r := NewRetryer(config)

	attempts := 0
	err := r.Do(context.Background(), func() error {
		attempts++
		if attempts < 3 {
			return &RetryableError{Err: errors.New("temporary failure")}
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected no error after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got: %d", attempts)
	}
}

func TestRetryer_DoesNotRetryNonRetryableError(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
		JitterFactor:   0.0,
	}
	r := NewRetryer(config)

	attempts := 0
	err := r.Do(context.Background(), func() error {
		attempts++
		return errors.New("permanent failure")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got: %d", attempts)
	}
}

func TestRetryer_RespectsMaxAttempts(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     100 * time.Millisecond,
		Multiplier:     2.0,
		JitterFactor:   0.0,
	}
	r := NewRetryer(config)

	attempts := 0
	err := r.Do(context.Background(), func() error {
		attempts++
		return &RetryableError{Err: errors.New("always fails")}
	})

	if err == nil {
		t.Error("expected error after max attempts, got nil")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got: %d", attempts)
	}
}

func TestRetryer_RespectsContextCancellation(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:    10,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		JitterFactor:   0.0,
	}
	r := NewRetryer(config)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	attempts := 0
	err := r.Do(ctx, func() error {
		attempts++
		return &RetryableError{Err: errors.New("always fails")}
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context deadline exceeded, got: %v", err)
	}
	if attempts > 2 {
		t.Errorf("expected at most 2 attempts before context cancellation, got: %d", attempts)
	}
}

func TestIsRetryable_RetryableError(t *testing.T) {
	err := &RetryableError{Err: errors.New("some error")}
	if !IsRetryable(err) {
		t.Error("expected RetryableError to be retryable")
	}
}

func TestIsRetryable_ContextErrors(t *testing.T) {
	if IsRetryable(context.Canceled) {
		t.Error("expected context.Canceled to not be retryable")
	}
	if IsRetryable(context.DeadlineExceeded) {
		t.Error("expected context.DeadlineExceeded to not be retryable")
	}
}

func TestIsRetryable_CircuitOpen(t *testing.T) {
	if IsRetryable(ErrCircuitOpen) {
		t.Error("expected ErrCircuitOpen to not be retryable")
	}
}

func TestIsRetryable_MessagePatterns(t *testing.T) {
	retryableErrors := []string{
		"rate limit exceeded",
		"too many requests",
		"service unavailable",
		"internal server error",
		"connection timeout",
		"connection reset by peer",
	}

	for _, msg := range retryableErrors {
		err := errors.New(msg)
		if !IsRetryable(err) {
			t.Errorf("expected error %q to be retryable", msg)
		}
	}
}

func TestIsRetryable_NonRetryableMessages(t *testing.T) {
	nonRetryableErrors := []string{
		"invalid input",
		"not found",
		"permission denied",
	}

	for _, msg := range nonRetryableErrors {
		err := errors.New(msg)
		if IsRetryable(err) {
			t.Errorf("expected error %q to not be retryable", msg)
		}
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	retryableCodes := []int{
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusBadGateway,
		http.StatusInternalServerError,
	}

	for _, code := range retryableCodes {
		if !IsRetryableStatusCode(code) {
			t.Errorf("expected status code %d to be retryable", code)
		}
	}

	nonRetryableCodes := []int{
		http.StatusOK,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
	}

	for _, code := range nonRetryableCodes {
		if IsRetryableStatusCode(code) {
			t.Errorf("expected status code %d to not be retryable", code)
		}
	}
}

func TestRetryer_DefaultConfigs(t *testing.T) {
	phase1 := DefaultPhase1RetryConfig()
	if phase1.MaxAttempts != 3 {
		t.Errorf("expected phase1 MaxAttempts to be 3, got %d", phase1.MaxAttempts)
	}
	if phase1.InitialBackoff != 2*time.Second {
		t.Errorf("expected phase1 InitialBackoff to be 2s, got %v", phase1.InitialBackoff)
	}

	phase2 := DefaultPhase2RetryConfig()
	if phase2.MaxAttempts != 2 {
		t.Errorf("expected phase2 MaxAttempts to be 2, got %d", phase2.MaxAttempts)
	}
	if phase2.InitialBackoff != 1*time.Second {
		t.Errorf("expected phase2 InitialBackoff to be 1s, got %v", phase2.InitialBackoff)
	}
}

func TestRetryableError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &RetryableError{Err: inner}

	if !errors.Is(err, inner) {
		t.Error("expected RetryableError to unwrap to inner error")
	}
}
