// Package mqtt is a secondary adapter that collects readings from an MQTT
// broker. The broker client is abstracted so tests can inject a fake publisher
// and no live broker is required.
package mqtt

import (
	"encoding/json"
	"errors"
	"time"

	"industrial-gateway/internal/domain"
)

// Client is the minimal broker abstraction the adapter needs.
type Client interface {
	Subscribe(topic string, handler func([]byte)) error
	Disconnect()
}

// rawMessage is the JSON shape expected on the wire.
type rawMessage struct {
	DeviceID  string  `json:"device_id"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Timestamp string  `json:"timestamp,omitempty"`
}

// Collector implements domain.CollectorPort over an MQTT client.
type Collector struct {
	client Client
	topic  string
	queue  []domain.Reading
}

func NewCollector(client Client, topic string) *Collector {
	return &Collector{client: client, topic: topic}
}

// Start wires the broker handler that pushes decoded readings into a queue.
func (c *Collector) Start() error {
	if c.client == nil {
		return errors.New("mqtt client is nil")
	}
	return c.client.Subscribe(c.topic, c.onMessage)
}

func (c *Collector) onMessage(payload []byte) {
	var m rawMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return
	}
	ts, err := time.Parse(time.RFC3339, m.Timestamp)
	if err != nil || m.Timestamp == "" {
		ts = time.Now().UTC()
	}
	c.queue = append(c.queue, domain.Reading{
		DeviceID: m.DeviceID, Metric: m.Metric, Value: m.Value,
		Unit: m.Unit, Timestamp: ts, Source: "mqtt", Quality: "good",
	})
}

// Collect drains the queue and returns buffered readings.
func (c *Collector) Collect() ([]domain.Reading, error) {
	out := c.queue
	c.queue = nil
	return out, nil
}

func (c *Collector) Name() string { return "mqtt-collector" }

// IngestRaw is a helper for tests to push messages directly (no broker).
func (c *Collector) IngestRaw(payload []byte) { c.onMessage(payload) }
