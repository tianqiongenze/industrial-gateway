// Package usecase wires the domain logic to the ports (collectors + repository).
package usecase

import (
	"context"
	"fmt"
	"log"

	"industrial-gateway/internal/domain"
)

// IngestService is the primary use case: collect from a source, transform,
// persist, classify and cache. It is constructed from ports only.
type IngestService struct {
	collectors []domain.CollectorPort
	pipeline   *domain.Pipeline
	repo       domain.RepositoryPort
	cache      domain.CachePort
}

func NewIngestService(pipeline *domain.Pipeline, repo domain.RepositoryPort,
	cache domain.CachePort, collectors ...domain.CollectorPort) *IngestService {
	return &IngestService{
		collectors: collectors, pipeline: pipeline, repo: repo, cache: cache,
	}
}

// RunOnce pulls one batch from every collector, transforms, stores & caches.
// Returns the count of successfully ingested readings.
func (s *IngestService) RunOnce(ctx context.Context) (int, error) {
	total := 0
	for _, c := range s.collectors {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		raws, err := c.Collect()
		if err != nil {
			log.Printf("collector %s error: %v", c.Name(), err)
			continue
		}
		for _, raw := range raws {
			r, err := s.pipeline.Apply(raw)
			if err != nil {
				log.Printf("transform error for %s: %v", raw.DeviceID, err)
				continue
			}
			if err := s.repo.Save(r); err != nil {
				log.Printf("save error for %s: %v", r.DeviceID, err)
				continue
			}
			status := domain.Classify(r)
			s.cache.Set(fmt.Sprintf("status:%s", r.DeviceID), status, 0)
			total++
		}
	}
	return total, nil
}

// StatusFor returns the cached status of a device.
func (s *IngestService) StatusFor(deviceID string) string {
	if v, ok := s.cache.Get("status:" + deviceID); ok {
		return v
	}
	return "UNKNOWN"
}
