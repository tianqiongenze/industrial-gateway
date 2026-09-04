package memoryrepo

import (
	"testing"
	"time"

	"industrial-gateway/internal/domain"
)

func TestRepository_SaveAndCount(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	rd := domain.Reading{DeviceID: "dev-1", Metric: "temperature", Value: 50, Unit: "C", Timestamp: now, Source: "rest", Quality: "good"}
	if err := r.Save(rd); err != nil {
		t.Fatalf("save: %v", err)
	}
	if c, _ := r.Count(""); c != 1 {
		t.Errorf("count = %d, want 1", c)
	}
}

func TestRepository_InvalidReadingRejected(t *testing.T) {
	r := New()
	err := r.Save(domain.Reading{DeviceID: "", Metric: "temperature", Value: 1, Timestamp: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error for invalid reading")
	}
}

func TestRepository_LatestFiltersAndOrders(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	for i, v := range []float64{10, 20, 30} {
		_ = r.Save(domain.Reading{DeviceID: "dev-1", Metric: "temperature", Value: v, Unit: "C", Timestamp: now.Add(time.Duration(i) * time.Minute), Source: "rest", Quality: "good"})
	}
	// different device should be excluded
	_ = r.Save(domain.Reading{DeviceID: "dev-2", Metric: "temperature", Value: 99, Unit: "C", Timestamp: now, Source: "rest", Quality: "good"})

	got, _ := r.Latest("dev-1", "temperature", 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Value != 20 || got[1].Value != 30 {
		t.Errorf("order/values = %v, want [20 30] chronological", []float64{got[0].Value, got[1].Value})
	}
}

func TestRepository_CountByDevice(t *testing.T) {
	r := New()
	now := time.Now().UTC()
	_ = r.Save(domain.Reading{DeviceID: "a", Metric: "m", Value: 1, Timestamp: now, Source: "s", Quality: "good"})
	_ = r.Save(domain.Reading{DeviceID: "b", Metric: "m", Value: 1, Timestamp: now, Source: "s", Quality: "good"})
	_ = r.Save(domain.Reading{DeviceID: "a", Metric: "m", Value: 1, Timestamp: now, Source: "s", Quality: "good"})
	if c, _ := r.Count("a"); c != 2 {
		t.Errorf("count(a) = %d, want 2", c)
	}
	if c, _ := r.Count("b"); c != 1 {
		t.Errorf("count(b) = %d, want 1", c)
	}
}
