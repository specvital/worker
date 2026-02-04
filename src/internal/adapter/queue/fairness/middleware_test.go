package fairness

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"
)

// mockTierResolver returns tier from job args for testing.
// This allows tests to control tier via job args like before the refactor.
type mockTierResolver struct{}

func (m *mockTierResolver) ResolveTier(_ context.Context, userID string) PlanTier {
	return TierFree
}

// argsBasedTierResolver extracts tier from job args for backward compatibility in tests.
type argsBasedTierResolver struct {
	tiers map[string]PlanTier
}

func newArgsBasedTierResolver() *argsBasedTierResolver {
	return &argsBasedTierResolver{
		tiers: make(map[string]PlanTier),
	}
}

func (r *argsBasedTierResolver) SetTier(userID string, tier PlanTier) {
	r.tiers[userID] = tier
}

func (r *argsBasedTierResolver) ResolveTier(_ context.Context, userID string) PlanTier {
	if tier, ok := r.tiers[userID]; ok {
		return tier
	}
	return TierFree
}

// setupMiddleware creates a FairnessMiddleware with default test configuration.
func setupMiddleware(t *testing.T) (*FairnessMiddleware, *PerUserLimiter, *Config) {
	t.Helper()
	return setupMiddlewareWithResolver(t, &mockTierResolver{})
}

// setupMiddlewareWithResolver creates a FairnessMiddleware with a custom TierResolver.
func setupMiddlewareWithResolver(t *testing.T, resolver TierResolver) (*FairnessMiddleware, *PerUserLimiter, *Config) {
	t.Helper()
	cfg := &Config{
		FreeConcurrentLimit:       1,
		ProConcurrentLimit:        3,
		EnterpriseConcurrentLimit: 5,
		SnoozeDuration:            30 * time.Second,
		SnoozeJitter:              10 * time.Second,
	}
	limiter, err := NewPerUserLimiter(cfg)
	if err != nil {
		t.Fatalf("NewPerUserLimiter failed: %v", err)
	}
	extractor := &JSONArgsExtractor{}
	middleware := NewFairnessMiddleware(limiter, extractor, resolver, cfg)
	return middleware, limiter, cfg
}

func TestFairnessMiddleware_Work_UnderLimit(t *testing.T) {
	middleware, limiter, _ := setupMiddleware(t)

	job := &rivertype.JobRow{
		ID:          1,
		EncodedArgs: []byte(`{"user_id":"user1","tier":"free"}`),
	}

	innerCalled := false
	doInner := func(ctx context.Context) error {
		innerCalled = true
		return nil
	}

	err := middleware.Work(context.Background(), job, doInner)
	if err != nil {
		t.Errorf("Work failed: %v", err)
	}
	if !innerCalled {
		t.Error("doInner was not called when under limit")
	}
	if limiter.ActiveCount("user1") != 0 {
		t.Errorf("ActiveCount after Work = %d, want 0 (released)", limiter.ActiveCount("user1"))
	}
}

func TestFairnessMiddleware_Work_OverLimit(t *testing.T) {
	middleware, limiter, _ := setupMiddleware(t)

	// Acquire first slot
	if !limiter.TryAcquire("user1", TierFree, 1) {
		t.Fatal("TryAcquire for job1 failed")
	}

	// Attempt second job (should snooze)
	job2 := &rivertype.JobRow{
		ID:          2,
		EncodedArgs: []byte(`{"user_id":"user1","tier":"free"}`),
	}

	innerCalled := false
	doInner := func(ctx context.Context) error {
		innerCalled = true
		return nil
	}

	err := middleware.Work(context.Background(), job2, doInner)
	if err == nil {
		t.Fatal("Work should return error when over limit")
	}
	if !strings.Contains(err.Error(), "snooze") && !strings.Contains(err.Error(), "Snooze") {
		t.Errorf("Expected JobSnooze error, got: %v", err)
	}
	if innerCalled {
		t.Error("doInner was called when over limit")
	}
}

func TestFairnessMiddleware_Work_EmptyUserID(t *testing.T) {
	middleware, _, _ := setupMiddleware(t)

	job := &rivertype.JobRow{
		ID:          1,
		EncodedArgs: []byte(`{}`), // no user_id
	}

	innerCalled := false
	doInner := func(ctx context.Context) error {
		innerCalled = true
		return nil
	}

	err := middleware.Work(context.Background(), job, doInner)
	if err != nil {
		t.Errorf("Work failed for system job: %v", err)
	}
	if !innerCalled {
		t.Error("doInner was not called for system job")
	}
}

func TestFairnessMiddleware_Work_ReleaseOnPanic(t *testing.T) {
	middleware, limiter, _ := setupMiddleware(t)

	job := &rivertype.JobRow{
		ID:          1,
		EncodedArgs: []byte(`{"user_id":"user1","tier":"free"}`),
	}

	doInner := func(ctx context.Context) error {
		panic("test panic")
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic but did not occur")
			}
		}()
		_ = middleware.Work(context.Background(), job, doInner)
	}()

	// Check slot released AFTER panic was handled
	if limiter.ActiveCount("user1") != 0 {
		t.Errorf("ActiveCount after panic = %d, want 0 (released)", limiter.ActiveCount("user1"))
	}
}

func TestFairnessMiddleware_Work_ReleaseOnError(t *testing.T) {
	middleware, limiter, _ := setupMiddleware(t)

	job := &rivertype.JobRow{
		ID:          1,
		EncodedArgs: []byte(`{"user_id":"user1","tier":"free"}`),
	}

	expectedErr := errors.New("test error")
	doInner := func(ctx context.Context) error {
		return expectedErr
	}

	err := middleware.Work(context.Background(), job, doInner)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Work should propagate error, got: %v", err)
	}
	if limiter.ActiveCount("user1") != 0 {
		t.Errorf("ActiveCount after error = %d, want 0 (released)", limiter.ActiveCount("user1"))
	}
}

func TestFairnessMiddleware_Work_ProTierLimit(t *testing.T) {
	resolver := newArgsBasedTierResolver()
	resolver.SetTier("user1", TierPro)
	middleware, limiter, _ := setupMiddlewareWithResolver(t, resolver)

	doInner := func(ctx context.Context) error {
		return nil
	}

	// Acquire 3 slots (Pro tier limit)
	for i := 1; i <= 3; i++ {
		job := &rivertype.JobRow{
			ID:          int64(i),
			EncodedArgs: []byte(`{"user_id":"user1"}`),
		}
		err := middleware.Work(context.Background(), job, doInner)
		if err != nil {
			t.Errorf("Work %d failed: %v", i, err)
		}
	}

	// 4th job should be snoozed
	if !limiter.TryAcquire("user1", TierPro, 101) {
		t.Fatal("TryAcquire for slot 1 failed")
	}
	if !limiter.TryAcquire("user1", TierPro, 102) {
		t.Fatal("TryAcquire for slot 2 failed")
	}
	if !limiter.TryAcquire("user1", TierPro, 103) {
		t.Fatal("TryAcquire for slot 3 failed")
	}

	job4 := &rivertype.JobRow{
		ID:          104,
		EncodedArgs: []byte(`{"user_id":"user1"}`),
	}

	err := middleware.Work(context.Background(), job4, doInner)
	if err == nil {
		t.Error("4th Pro tier job should be snoozed")
	}
	if !strings.Contains(err.Error(), "snooze") && !strings.Contains(err.Error(), "Snooze") {
		t.Errorf("Expected JobSnooze error, got: %v", err)
	}
}

func TestFairnessMiddleware_Work_EnterpriseTierLimit(t *testing.T) {
	resolver := newArgsBasedTierResolver()
	resolver.SetTier("user1", TierEnterprise)
	middleware, limiter, _ := setupMiddlewareWithResolver(t, resolver)

	doInner := func(ctx context.Context) error {
		return nil
	}

	// Acquire 5 slots (Enterprise tier limit)
	for i := 1; i <= 5; i++ {
		if !limiter.TryAcquire("user1", TierEnterprise, int64(i)) {
			t.Fatalf("TryAcquire for slot %d failed", i)
		}
	}

	// 6th job should be snoozed
	job6 := &rivertype.JobRow{
		ID:          6,
		EncodedArgs: []byte(`{"user_id":"user1"}`),
	}

	err := middleware.Work(context.Background(), job6, doInner)
	if err == nil {
		t.Error("6th Enterprise tier job should be snoozed")
	}
	if !strings.Contains(err.Error(), "snooze") && !strings.Contains(err.Error(), "Snooze") {
		t.Errorf("Expected JobSnooze error, got: %v", err)
	}
}

func TestFairnessMiddleware_Work_UnknownUserDefaultsToFree(t *testing.T) {
	// TierResolver returns TierFree for unknown users (default behavior)
	middleware, limiter, _ := setupMiddleware(t)

	// Acquire 1 slot with free tier (limit=1)
	if !limiter.TryAcquire("user1", TierFree, 1) {
		t.Fatal("TryAcquire for slot 1 failed")
	}

	// 2nd job should be snoozed (Free tier limit=1)
	job2 := &rivertype.JobRow{
		ID:          2,
		EncodedArgs: []byte(`{"user_id":"user1"}`),
	}

	doInner := func(ctx context.Context) error {
		return nil
	}

	err := middleware.Work(context.Background(), job2, doInner)
	if err == nil {
		t.Error("2nd job should be snoozed (Free tier limit)")
	}
	if !strings.Contains(err.Error(), "snooze") && !strings.Contains(err.Error(), "Snooze") {
		t.Errorf("Expected JobSnooze error, got: %v", err)
	}
}
