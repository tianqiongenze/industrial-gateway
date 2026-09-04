// Package domain holds the pure business model: a device reading and the
// transformation pipeline. It depends on NOTHING outside the standard library,
// which is the whole point of hexagonal architecture.
package domain

import (
	"errors"
	"math"
	"time"
)

// Reading is a single normalised measurement collected from a field device.
type Reading struct {
	DeviceID  string    `json:"device_id"`
	Metric    string    `json:"metric"`     // temperature, vibration, pressure, ...
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // mqtt, modbus
	Quality   string    `json:"quality"` // good, bad, unknown
}

// Validate enforces the invariants of a collected reading.
func (r Reading) Validate() error {
	if r.DeviceID == "" {
		return errors.New("device_id is required")
	}
	if r.Metric == "" {
		return errors.New("metric is required")
	}
	if math.IsNaN(r.Value) || math.IsInf(r.Value, 0) {
		return errors.New("value must be a finite number")
	}
	if r.Timestamp.IsZero() {
		return errors.New("timestamp is required")
	}
	return nil
}

// DeviceStatus summarises a device's health derived from its latest readings.
type DeviceStatus struct {
	DeviceID   string    `json:"device_id"`
	Status     string    `json:"status"` // NORMAL, WARNING, CRITICAL
	LastValue  float64   `json:"last_value"`
	Metric     string    `json:"metric"`
	Readings   int       `json:"readings"`
	LastSeen   time.Time `json:"last_seen"`
}

// CollectorPort is the port (interface) every device-data collector must
// implement (MQTT adapter, Modbus adapter, ...).
type CollectorPort interface {
	Collect() ([]Reading, error)
	Name() string
}

// RepositoryPort is the persistence port (in-memory, Redis, ...).
type RepositoryPort interface {
	Save(Reading) error
	Latest(deviceID, metric string, n int) ([]Reading, error)
	Count(deviceID string) (int, error)
}

// CachePort is the live-status cache port.
type CachePort interface {
	Get(key string) (string, bool)
	Set(key, value string, ttl time.Duration)
}
