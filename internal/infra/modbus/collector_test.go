package modbus

import (
	"math"
	"testing"

	"industrial-gateway/internal/domain"
)

type fakeClient struct {
	regs map[uint16][]uint16
}

func (f *fakeClient) ReadHoldingRegisters(address, quantity uint16) ([]uint16, error) {
	return f.regs[address], nil
}
func (f *fakeClient) Close() error { return nil }

func floatEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCollector_PollsMaps(t *testing.T) {
	fc := &fakeClient{regs: map[uint16][]uint16{100: {750}, 200: {3}}}
	maps := []RegisterMap{
		{Address: 100, Quantity: 1, DeviceID: "plc-1", Metric: "temperature", Unit: "C", Scale: 0.1},
		{Address: 200, Quantity: 1, DeviceID: "plc-1", Metric: "vibration", Unit: "mm/s", Scale: 0.1},
	}
	c := NewCollector(fc, maps)
	out, err := c.Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	want := map[string]float64{"temperature": 75, "vibration": 0.3}
	for _, rd := range out {
		got, ok := want[rd.Metric]
		if !ok {
			t.Errorf("unexpected metric %s", rd.Metric)
			continue
		}
		if !floatEq(rd.Value, got) {
			t.Errorf("%s value = %v, want %v", rd.Metric, rd.Value, got)
		}
		if rd.Source != "modbus" {
			t.Errorf("source = %q, want modbus", rd.Source)
		}
		if domain.Classify(rd) == "" {
			t.Error("classify returned empty")
		}
	}
}

func TestCollector_NilClientErrors(t *testing.T) {
	c := NewCollector(nil, nil)
	if _, err := c.Collect(); err == nil {
		t.Fatal("expected error with nil client")
	}
}
