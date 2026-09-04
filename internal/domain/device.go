package domain

import (
	"errors"
	"strings"
	"time"
)

// Device is the aggregate root for gateway device management.
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Location  string    `json:"location"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate enforces device invariants.
func (d *Device) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("device name is required")
	}
	if strings.TrimSpace(d.Type) == "" {
		return errors.New("device type is required")
	}
	return nil
}

// DeviceStorePort is the persistence port for device CRUD.
// Implementations MUST route writes (Create/Update/Delete) to the WRITE
// database and reads (Get/List/Count) to the READ database.
type DeviceStorePort interface {
	CreateDevice(d *Device) error
	GetDevice(id string) (*Device, error)
	ListDevices() ([]Device, error)
	UpdateDevice(id string, d *Device) error
	DeleteDevice(id string) error
	CountDevices() (int64, error)
}

// ErrNotFound is returned when an entity does not exist.
var ErrNotFound = errors.New("entity not found")
