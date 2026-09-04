package grpcadapter

import (
	"context"
	"testing"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/infra/memorycache"
	"industrial-gateway/internal/infra/memoryrepo"
	"industrial-gateway/internal/usecase"
)

func newService() *Service {
	repo := memoryrepo.New()
	cache := memorycache.New()
	uc := usecase.NewIngestService(
		&domain.Pipeline{Steps: []*domain.Transformer{domain.NewTransformer(nil, nil)}},
		repo, cache)
	return NewService(uc)
}

func TestService_Ingest(t *testing.T) {
	s := newService()
	resp, err := s.Ingest(context.Background(), IngestRequest{
		DeviceID: "pump-1", Metric: "temperature", Value: 96, Unit: "C", Source: "grpc"})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if resp.Status != "CRITICAL" {
		t.Errorf("status = %q, want CRITICAL", resp.Status)
	}
	if !resp.Saved {
		t.Error("expected Saved=true")
	}
}

func TestService_Ingest_MissingDevice(t *testing.T) {
	s := newService()
	if _, err := s.Ingest(context.Background(), IngestRequest{Metric: "temperature", Value: 1}); err == nil {
		t.Fatal("expected error for missing device_id")
	}
}

func TestService_Ingest_InvalidValue(t *testing.T) {
	s := newService()
	// NaN value should fail validation in domain.Reading.Validate
	_, err := s.Ingest(context.Background(), IngestRequest{
		DeviceID: "d", Metric: "temperature", Value: 1})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
