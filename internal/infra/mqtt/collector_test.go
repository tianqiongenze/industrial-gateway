package mqtt

import (
	"encoding/json"
	"testing"

	"industrial-gateway/internal/domain"
)

// fakeClient records the handler so we can drive it without a broker.
type fakeClient struct {
	handler func([]byte)
}

func (f *fakeClient) Subscribe(topic string, handler func([]byte)) error {
	f.handler = handler
	return nil
}
func (f *fakeClient) Disconnect() {}

func TestCollector_DecodesAndQueues(t *testing.T) {
	fc := &fakeClient{}
	c := NewCollector(fc, "industrial/+/+")
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	msg, _ := json.Marshal(map[string]interface{}{
		"device_id": "pump-1", "metric": "vibration", "value": 6.5, "unit": "mm/s"})
	fc.handler(msg)
	out, err := c.Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1", len(out))
	}
	rd := out[0]
	if rd.DeviceID != "pump-1" || rd.Metric != "vibration" || rd.Value != 6.5 {
		t.Errorf("decoded reading = %+v", rd)
	}
	if rd.Source != "mqtt" {
		t.Errorf("source = %q, want mqtt", rd.Source)
	}
	if domain.Classify(rd) != "WARNING" {
		t.Errorf("classify = %q, want WARNING", domain.Classify(rd))
	}
}

func TestCollector_CollectDrainsQueue(t *testing.T) {
	fc := &fakeClient{}
	c := NewCollector(fc, "t")
	_ = c.Start()
	c.IngestRaw([]byte(`{"device_id":"d","metric":"temperature","value":30,"unit":"C"}`))
	first, _ := c.Collect()
	if len(first) != 1 {
		t.Fatalf("first len = %d, want 1", len(first))
	}
	second, _ := c.Collect()
	if len(second) != 0 {
		t.Errorf("second len = %d, want 0 (drained)", len(second))
	}
}

func TestCollector_NilClientStartErrors(t *testing.T) {
	c := NewCollector(nil, "t")
	if err := c.Start(); err == nil {
		t.Fatal("expected error with nil client")
	}
}
