// Package memoryrepo is an in-memory implementation of domain.RepositoryPort.
// Used by tests and as a zero-dependency default in dev mode.
package memoryrepo

import (
	"sync"

	"industrial-gateway/internal/domain"
)

type Repository struct {
	mu       sync.Mutex
	readings []domain.Reading
}

func New() *Repository { return &Repository{} }

func (r *Repository) Save(rd domain.Reading) error {
	if err := rd.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readings = append(r.readings, rd)
	return nil
}

func (r *Repository) Latest(deviceID, metric string, n int) ([]domain.Reading, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Reading
	for i := len(r.readings) - 1; i >= 0 && len(out) < n; i-- {
		rd := r.readings[i]
		if rd.DeviceID == deviceID && (metric == "" || rd.Metric == metric) {
			out = append([]domain.Reading{rd}, out...)
		}
	}
	return out, nil
}

func (r *Repository) Count(deviceID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if deviceID == "" {
		return len(r.readings), nil
	}
	c := 0
	for _, rd := range r.readings {
		if rd.DeviceID == deviceID {
			c++
		}
	}
	return c, nil
}
