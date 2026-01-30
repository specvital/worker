package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadQueueConfig_Defaults(t *testing.T) {
	clearQueueEnvVars(t)

	cfg := loadQueueConfig()

	if cfg.Analyzer.Priority != defaultAnalyzerPriorityWorkers {
		t.Errorf("Analyzer.Priority = %d, want %d", cfg.Analyzer.Priority, defaultAnalyzerPriorityWorkers)
	}
	if cfg.Analyzer.Default != defaultAnalyzerDefaultWorkers {
		t.Errorf("Analyzer.Default = %d, want %d", cfg.Analyzer.Default, defaultAnalyzerDefaultWorkers)
	}
	if cfg.Analyzer.Scheduled != defaultAnalyzerScheduledWorkers {
		t.Errorf("Analyzer.Scheduled = %d, want %d", cfg.Analyzer.Scheduled, defaultAnalyzerScheduledWorkers)
	}
	if cfg.Specgen.Priority != defaultSpecgenPriorityWorkers {
		t.Errorf("Specgen.Priority = %d, want %d", cfg.Specgen.Priority, defaultSpecgenPriorityWorkers)
	}
	if cfg.Specgen.Default != defaultSpecgenDefaultWorkers {
		t.Errorf("Specgen.Default = %d, want %d", cfg.Specgen.Default, defaultSpecgenDefaultWorkers)
	}
	if cfg.Specgen.Scheduled != defaultSpecgenScheduledWorkers {
		t.Errorf("Specgen.Scheduled = %d, want %d", cfg.Specgen.Scheduled, defaultSpecgenScheduledWorkers)
	}
}

func TestLoadQueueConfig_EnvOverride(t *testing.T) {
	clearQueueEnvVars(t)
	t.Setenv("ANALYZER_QUEUE_PRIORITY_WORKERS", "10")
	t.Setenv("ANALYZER_QUEUE_DEFAULT_WORKERS", "7")
	t.Setenv("SPECGEN_QUEUE_SCHEDULED_WORKERS", "4")

	cfg := loadQueueConfig()

	if cfg.Analyzer.Priority != 10 {
		t.Errorf("Analyzer.Priority = %d, want 10", cfg.Analyzer.Priority)
	}
	if cfg.Analyzer.Default != 7 {
		t.Errorf("Analyzer.Default = %d, want 7", cfg.Analyzer.Default)
	}
	if cfg.Specgen.Scheduled != 4 {
		t.Errorf("Specgen.Scheduled = %d, want 4", cfg.Specgen.Scheduled)
	}
	// Non-overridden values should use defaults
	if cfg.Analyzer.Scheduled != defaultAnalyzerScheduledWorkers {
		t.Errorf("Analyzer.Scheduled = %d, want %d", cfg.Analyzer.Scheduled, defaultAnalyzerScheduledWorkers)
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue int
		want         int
	}{
		{"empty uses default", "", 5, 5},
		{"valid number", "10", 5, 10},
		{"invalid string uses default", "abc", 5, 5},
		{"negative uses default", "-1", 5, 5},
		{"zero uses default", "0", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_ENV_INT"
			if tt.envValue != "" {
				t.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}

			got := getEnvInt(key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvInt(%q, %d) = %d, want %d", tt.envValue, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestLoadFairnessConfig_Defaults(t *testing.T) {
	clearFairnessEnvVars(t)

	cfg := loadFairnessConfig()

	if !cfg.Enabled {
		t.Error("Enabled should default to true")
	}
	if cfg.FreeConcurrentLimit != 1 {
		t.Errorf("FreeConcurrentLimit = %d, want 1", cfg.FreeConcurrentLimit)
	}
	if cfg.ProConcurrentLimit != 3 {
		t.Errorf("ProConcurrentLimit = %d, want 3", cfg.ProConcurrentLimit)
	}
	if cfg.EnterpriseConcurrentLimit != 5 {
		t.Errorf("EnterpriseConcurrentLimit = %d, want 5", cfg.EnterpriseConcurrentLimit)
	}
	if cfg.SnoozeDuration != 30*time.Second {
		t.Errorf("SnoozeDuration = %v, want 30s", cfg.SnoozeDuration)
	}
	if cfg.SnoozeJitter != 10*time.Second {
		t.Errorf("SnoozeJitter = %v, want 10s", cfg.SnoozeJitter)
	}
}

func TestLoadFairnessConfig_EnvOverride(t *testing.T) {
	clearFairnessEnvVars(t)
	t.Setenv("FAIRNESS_ENABLED", "false")
	t.Setenv("FAIRNESS_FREE_LIMIT", "2")
	t.Setenv("FAIRNESS_PRO_LIMIT", "5")
	t.Setenv("FAIRNESS_ENTERPRISE_LIMIT", "10")
	t.Setenv("FAIRNESS_SNOOZE_DURATION", "1m")
	t.Setenv("FAIRNESS_SNOOZE_JITTER", "20s")

	cfg := loadFairnessConfig()

	if cfg.Enabled {
		t.Error("Enabled should be false")
	}
	if cfg.FreeConcurrentLimit != 2 {
		t.Errorf("FreeConcurrentLimit = %d, want 2", cfg.FreeConcurrentLimit)
	}
	if cfg.ProConcurrentLimit != 5 {
		t.Errorf("ProConcurrentLimit = %d, want 5", cfg.ProConcurrentLimit)
	}
	if cfg.EnterpriseConcurrentLimit != 10 {
		t.Errorf("EnterpriseConcurrentLimit = %d, want 10", cfg.EnterpriseConcurrentLimit)
	}
	if cfg.SnoozeDuration != 60*time.Second {
		t.Errorf("SnoozeDuration = %v, want 1m", cfg.SnoozeDuration)
	}
	if cfg.SnoozeJitter != 20*time.Second {
		t.Errorf("SnoozeJitter = %v, want 20s", cfg.SnoozeJitter)
	}
}

func TestLoadFairnessConfig_DisabledSkipsValidation(t *testing.T) {
	clearFairnessEnvVars(t)
	t.Setenv("FAIRNESS_ENABLED", "false")
	t.Setenv("FAIRNESS_FREE_LIMIT", "-1") // Invalid but should not panic when disabled

	cfg := loadFairnessConfig()

	if cfg.Enabled {
		t.Error("Enabled should be false")
	}
	// Should not panic even with invalid values when disabled
}

func TestLoadFairnessConfig_ValidationPanics(t *testing.T) {
	tests := []struct {
		name     string
		envSetup func(*testing.T)
	}{
		{
			name: "zero snooze duration",
			envSetup: func(t *testing.T) {
				t.Setenv("FAIRNESS_ENABLED", "true")
				t.Setenv("FAIRNESS_SNOOZE_DURATION", "0s")
			},
		},
		{
			name: "negative snooze duration",
			envSetup: func(t *testing.T) {
				t.Setenv("FAIRNESS_ENABLED", "true")
				t.Setenv("FAIRNESS_SNOOZE_DURATION", "-1s")
			},
		},
		{
			name: "negative snooze jitter",
			envSetup: func(t *testing.T) {
				t.Setenv("FAIRNESS_ENABLED", "true")
				t.Setenv("FAIRNESS_SNOOZE_JITTER", "-1s")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearFairnessEnvVars(t)
			tt.envSetup(t)

			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic but got none")
				}
			}()

			loadFairnessConfig()
		})
	}
}

func TestLoadFairnessConfig_IntValidationByGetEnvInt(t *testing.T) {
	clearFairnessEnvVars(t)
	t.Setenv("FAIRNESS_ENABLED", "true")
	t.Setenv("FAIRNESS_FREE_LIMIT", "0")
	t.Setenv("FAIRNESS_PRO_LIMIT", "-5")

	cfg := loadFairnessConfig()

	// getEnvInt filters out <=0, so should use defaults
	if cfg.FreeConcurrentLimit != 1 {
		t.Errorf("FreeConcurrentLimit = %d, want 1 (default used)", cfg.FreeConcurrentLimit)
	}
	if cfg.ProConcurrentLimit != 3 {
		t.Errorf("ProConcurrentLimit = %d, want 3 (default used)", cfg.ProConcurrentLimit)
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue bool
		want         bool
	}{
		{"empty uses default true", "", true, true},
		{"empty uses default false", "", false, false},
		{"valid true", "true", false, true},
		{"valid false", "false", true, false},
		{"valid 1", "1", false, true},
		{"valid 0", "0", true, false},
		{"invalid uses default", "invalid", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_ENV_BOOL"
			if tt.envValue != "" {
				t.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}

			got := getEnvBool(key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", tt.envValue, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func TestGetEnvDuration(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue time.Duration
		want         time.Duration
	}{
		{"empty uses default", "", 30 * time.Second, 30 * time.Second},
		{"valid duration seconds", "45s", 30 * time.Second, 45 * time.Second},
		{"valid duration minutes", "1m30s", 30 * time.Second, 90 * time.Second},
		{"invalid uses default", "invalid", 30 * time.Second, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_ENV_DURATION"
			if tt.envValue != "" {
				t.Setenv(key, tt.envValue)
			} else {
				os.Unsetenv(key)
			}

			got := getEnvDuration(key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvDuration(%q, %v) = %v, want %v", tt.envValue, tt.defaultValue, got, tt.want)
			}
		})
	}
}

func clearQueueEnvVars(t *testing.T) {
	t.Helper()
	envVars := []string{
		"ANALYZER_QUEUE_PRIORITY_WORKERS",
		"ANALYZER_QUEUE_DEFAULT_WORKERS",
		"ANALYZER_QUEUE_SCHEDULED_WORKERS",
		"SPECGEN_QUEUE_PRIORITY_WORKERS",
		"SPECGEN_QUEUE_DEFAULT_WORKERS",
		"SPECGEN_QUEUE_SCHEDULED_WORKERS",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}

func clearFairnessEnvVars(t *testing.T) {
	t.Helper()
	envVars := []string{
		"FAIRNESS_ENABLED",
		"FAIRNESS_FREE_LIMIT",
		"FAIRNESS_PRO_LIMIT",
		"FAIRNESS_ENTERPRISE_LIMIT",
		"FAIRNESS_SNOOZE_DURATION",
		"FAIRNESS_SNOOZE_JITTER",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}

func TestLoad_Phase1V2Flag_DefaultFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("ENCRYPTION_KEY", "test-key")
	os.Unsetenv("SPECVIEW_PHASE1_V2")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Phase1V2Enabled {
		t.Error("Phase1V2Enabled should default to false")
	}
}

func TestLoad_Phase1V2Flag_EnabledFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("ENCRYPTION_KEY", "test-key")
	t.Setenv("SPECVIEW_PHASE1_V2", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Phase1V2Enabled {
		t.Error("Phase1V2Enabled should be true when SPECVIEW_PHASE1_V2=true")
	}
}

func TestLoad_Phase1V3Flag_DefaultFalse(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("ENCRYPTION_KEY", "test-key")
	os.Unsetenv("SPECVIEW_PHASE1_V3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Phase1V3Enabled {
		t.Error("Phase1V3Enabled should default to false")
	}
}

func TestLoad_Phase1V3Flag_EnabledFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("ENCRYPTION_KEY", "test-key")
	t.Setenv("SPECVIEW_PHASE1_V3", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Phase1V3Enabled {
		t.Error("Phase1V3Enabled should be true when SPECVIEW_PHASE1_V3=true")
	}
}
