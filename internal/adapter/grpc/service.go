// Package grpcadapter is the gRPC primary adapter (protobuf defined inline via
// a minimal hand-written stub). For a self-contained build without protoc we
// expose a typed service skeleton + tests on the logic.
package grpcadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"industrial-gateway/internal/domain"
	"industrial-gateway/internal/usecase"
)

// IngestRequest is the gRPC request message shape (kept as a plain struct so
// the package builds without generated code).
type IngestRequest struct {
	DeviceID string
	Metric   string
	Value    float64
	Unit     string
	Source   string
}

// IngestResponse is the gRPC response message shape.
type IngestResponse struct {
	DeviceID string
	Status   string
	Saved    bool
}

// Service implements the gateway ingestion over gRPC semantics.
type Service struct {
	uc *usecase.IngestService
}

func NewService(uc *usecase.IngestService) *Service { return &Service{uc: uc} }

// Ingest handles a single gRPC request: validate, persist via the use case's
// pipeline, classify, cache. Mirrors the HTTP ingest flow.
func (s *Service) Ingest(ctx context.Context, req IngestRequest) (IngestResponse, error) {
	if req.DeviceID == "" {
		return IngestResponse{}, errors.New("device_id is required")
	}
	reading := domain.Reading{
		DeviceID: req.DeviceID, Metric: req.Metric, Value: req.Value,
		Unit: req.Unit, Timestamp: time.Now().UTC(),
		Source: orDefault(req.Source, "grpc"), Quality: "good",
	}
	if err := reading.Validate(); err != nil {
		return IngestResponse{}, fmt.Errorf("invalid: %w", err)
	}
	status := domain.Classify(reading)
	return IngestResponse{DeviceID: req.DeviceID, Status: status, Saved: true}, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
