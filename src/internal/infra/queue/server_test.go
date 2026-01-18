package queue

import (
	"testing"

	"github.com/riverqueue/river"
)

func TestBuildQueueConfig_MultiQueue(t *testing.T) {
	cfg := ServerConfig{
		Queues: []QueueAllocation{
			{Name: "analysis:priority", MaxWorkers: 5},
			{Name: "analysis:default", MaxWorkers: 3},
			{Name: "analysis:scheduled", MaxWorkers: 2},
		},
	}

	result := buildQueueConfig(cfg)

	if len(result) != 3 {
		t.Errorf("expected 3 queues, got %d", len(result))
	}

	tests := []struct {
		name       string
		maxWorkers int
	}{
		{"analysis:priority", 5},
		{"analysis:default", 3},
		{"analysis:scheduled", 2},
	}

	for _, tt := range tests {
		if q, ok := result[tt.name]; !ok {
			t.Errorf("queue %q not found", tt.name)
		} else if q.MaxWorkers != tt.maxWorkers {
			t.Errorf("queue %q: expected MaxWorkers %d, got %d", tt.name, tt.maxWorkers, q.MaxWorkers)
		}
	}
}

func TestBuildQueueConfig_MultiQueueWithZeroWorkers(t *testing.T) {
	cfg := ServerConfig{
		Queues: []QueueAllocation{
			{Name: "test:queue", MaxWorkers: 0},
		},
	}

	result := buildQueueConfig(cfg)

	if q, ok := result["test:queue"]; !ok {
		t.Error("queue not found")
	} else if q.MaxWorkers != DefaultConcurrency {
		t.Errorf("expected default concurrency %d, got %d", DefaultConcurrency, q.MaxWorkers)
	}
}

func TestBuildQueueConfig_LegacySingleQueue(t *testing.T) {
	cfg := ServerConfig{
		QueueName:   "legacy:queue",
		Concurrency: 10,
	}

	result := buildQueueConfig(cfg)

	if len(result) != 1 {
		t.Errorf("expected 1 queue, got %d", len(result))
	}

	if q, ok := result["legacy:queue"]; !ok {
		t.Error("legacy queue not found")
	} else if q.MaxWorkers != 10 {
		t.Errorf("expected MaxWorkers 10, got %d", q.MaxWorkers)
	}
}

func TestBuildQueueConfig_LegacyDefaults(t *testing.T) {
	cfg := ServerConfig{}

	result := buildQueueConfig(cfg)

	if q, ok := result[river.QueueDefault]; !ok {
		t.Errorf("expected default queue %q", river.QueueDefault)
	} else if q.MaxWorkers != DefaultConcurrency {
		t.Errorf("expected default concurrency %d, got %d", DefaultConcurrency, q.MaxWorkers)
	}
}

func TestBuildQueueConfig_QueuesOverridesLegacy(t *testing.T) {
	cfg := ServerConfig{
		Queues:      []QueueAllocation{{Name: "new:queue", MaxWorkers: 5}},
		QueueName:   "legacy:queue",
		Concurrency: 10,
	}

	result := buildQueueConfig(cfg)

	if _, ok := result["legacy:queue"]; ok {
		t.Error("legacy queue should not exist when Queues is set")
	}

	if _, ok := result["new:queue"]; !ok {
		t.Error("new queue should exist")
	}
}

func TestBuildQueueConfig_MultiQueueWithEmptyName(t *testing.T) {
	cfg := ServerConfig{
		Queues: []QueueAllocation{
			{Name: "", MaxWorkers: 5},
		},
	}

	result := buildQueueConfig(cfg)

	if q, ok := result[river.QueueDefault]; !ok {
		t.Errorf("expected empty name to fall back to default queue %q", river.QueueDefault)
	} else if q.MaxWorkers != 5 {
		t.Errorf("expected MaxWorkers 5, got %d", q.MaxWorkers)
	}
}
