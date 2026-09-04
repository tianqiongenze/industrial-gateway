// Package modbus is a secondary adapter that collects readings from PLCs over
// Modbus TCP. The transport is abstracted so unit tests inject a fake client.
package modbus

import (
	"errors"
	"time"

	"industrial-gateway/internal/domain"
)

// Client is the minimal Modbus client abstraction (read holding registers).
type Client interface {
	ReadHoldingRegisters(address, quantity uint16) ([]uint16, error)
	Close() error
}

// RegisterMap maps a register range to a metric + scaling + unit.
type RegisterMap struct {
	Address    uint16
	Quantity   uint16
	DeviceID   string
	Metric     string
	Unit       string
	Scale      float64 // raw register value * scale -> engineering value
	Offset     float64 // + offset
}

// Collector polls a set of register maps on a Modbus client.
type Collector struct {
	client Client
	maps   []RegisterMap
}

func NewCollector(client Client, maps []RegisterMap) *Collector {
	return &Collector{client: client, maps: maps}
}

// Read executes all configured register maps and returns raw readings.
func (c *Collector) pollMaps() ([]domain.Reading, error) {
	if c.client == nil {
		return nil, errors.New("modbus client is nil")
	}
	var out []domain.Reading
	for _, m := range c.maps {
		regs, err := c.client.ReadHoldingRegisters(m.Address, m.Quantity)
		if err != nil {
			return nil, err
		}
		// interpret the first register of the read as a uint16 -> float.
		for _, reg := range regs {
			val := float64(reg)
			if m.Scale != 0 {
				val = val*m.Scale + m.Offset
			}
			out = append(out, domain.Reading{
				DeviceID: m.DeviceID, Metric: m.Metric, Value: val,
				Unit: m.Unit, Timestamp: time.Now().UTC(),
				Source: "modbus", Quality: "good",
			})
			break // one reading per map in this simple model
		}
	}
	return out, nil
}

// Collect satisfies domain.CollectorPort.
func (c *Collector) Collect() ([]domain.Reading, error) { return c.pollMaps() }

func (c *Collector) Name() string { return "modbus-collector" }
