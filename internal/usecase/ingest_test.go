package usecase

import (
	"context"
	"testing"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/infra/memorycache"
	"industrial-gateway/internal/infra/memoryrepo"
)

// fakeCollector returns canned readings on Collect().
type fakeCollector struct {
	name string
	out  []domain.Reading
	err  error
}

func (f *fakeCollector) Collect() ([]domain.Reading, error) { return f.out, f.err }
func (f *fakeCollector) Name() string                          { return f.name }

func TestIngestService_RunOnce(t *testing.T) {
	repo := memoryrepo.New()
	cache := memorycache.New()
	uc := NewIngestService(
		&domain.Pipeline{Steps: []*domain.Transformer{domain.NewTransformer(map[string]string{"c": "Celsius"}, nil)}},
		repo, cache,
		&fakeCollector{name: "fake", out: []domain.Reading{
			{DeviceID: "d1", Metric: "temperature", Value: 96, Unit: "C", Timestamp: timeNow(), Source: "mqtt", Quality: "good"},
		}})
	n, err := uc.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("runonce: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}
	if s := uc.StatusFor("d1"); s != "CRITICAL" {
		t.Errorf("status = %q, want CRITICAL", s)
	}
}

func TestIngestService_StatusForUnknown(t *testing.T) {
	uc := NewIngestService(&domain.Pipeline{}, memoryrepo.New(), memorycache.New())
	if s := uc.StatusFor("ghost"); s != "UNKNOWN" {
		t.Errorf("status = %q, want UNKNOWN", s)
	}
}

func TestIngestService_InvalidReadingSkipped(t *testing.T) {
	repo := memoryrepo.New()
	cache := memorycache.New()
	uc := NewIngestService(
		&domain.Pipeline{Steps: []*domain.Transformer{domain.NewTransformer(nil, nil)}},
		repo, cache,
		&fakeCollector{name: "fake", out: []domain.Reading{
			{DeviceID: "", Metric: "temperature", Value: 1, Timestamp: timeNow()}, // invalid
		}})
	n, _ := uc.RunOnce(context.Background())
	if n != 0 {
		t.Errorf("count = %d, want 0 (invalid skipped)", n)
	}
}
