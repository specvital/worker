package analysis

import "time"

const hoursPerDay = 24

const (
	decayThreshold7Days  = 7
	decayThreshold14Days = 14
	decayThreshold30Days = 30
	decayThreshold60Days = 60
	decayThreshold90Days = 90
)

const (
	refreshInterval6Hours  = 6 * time.Hour
	refreshInterval12Hours = 12 * time.Hour
	refreshInterval24Hours = 24 * time.Hour
	refreshInterval3Days   = 3 * 24 * time.Hour
	refreshInterval7Days   = 7 * 24 * time.Hour
)

const maxConsecutiveFailures = 5

// CalculateRefreshIntervalAt returns refresh interval based on last viewed time.
// Returns 0 if lastViewedAt is future or idle exceeds 90 days.
func CalculateRefreshIntervalAt(lastViewedAt, now time.Time) time.Duration {
	duration := now.Sub(lastViewedAt)
	if duration < 0 {
		return 0
	}

	idleDays := int(duration.Hours() / hoursPerDay)

	switch {
	case idleDays <= decayThreshold7Days:
		return refreshInterval6Hours
	case idleDays <= decayThreshold14Days:
		return refreshInterval12Hours
	case idleDays <= decayThreshold30Days:
		return refreshInterval24Hours
	case idleDays <= decayThreshold60Days:
		return refreshInterval3Days
	case idleDays <= decayThreshold90Days:
		return refreshInterval7Days
	default:
		return 0
	}
}

// ShouldRefreshAt determines if codebase should be refreshed based on decay rules.
func ShouldRefreshAt(lastViewedAt time.Time, lastCompletedAt *time.Time, consecutiveFailures int, now time.Time) bool {
	if consecutiveFailures >= maxConsecutiveFailures {
		return false
	}

	interval := CalculateRefreshIntervalAt(lastViewedAt, now)
	if interval == 0 {
		return false
	}

	if lastCompletedAt == nil {
		return true
	}

	nextRefreshTime := lastCompletedAt.Add(interval)
	return now.After(nextRefreshTime) || now.Equal(nextRefreshTime)
}
