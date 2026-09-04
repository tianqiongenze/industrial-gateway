package domain

import (
	"math"
	"testing"
	"time"
)

func TestTransformer_Transform(t *testing.T) {
	now := time.Now().UTC()
	tf := NewTransformer(
		map[string]string{"c": "Celsius", "mm/s": "mm/s", "bar": "bar"},
		map[string]string{"rpm": "0.001"}, // raw counts -> rpm
	)
	cases := []struct {
		name   string
		in     Reading
		wantVal float64
		wantUnit string
	}{
		{"celsius normalise", Reading{"d", "temperature", 50, "C", now, "mqtt", "good"}, 50, "Celsius"},
		{"fahrenheit converts", Reading{"d", "temperature", 212, "F", now, "mqtt", "good"}, 100, "Celsius"},
		{"rpm scales", Reading{"d", "rpm", 12000, "", now, "modbus", "good"}, 12, "unknown"},
		{"empty unit -> unknown", Reading{"d", "temperature", 42, "", now, "rest", "good"}, 42, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tf.Transform(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !floatEq(out.Value, tc.wantVal) {
				t.Errorf("value = %v, want %v", out.Value, tc.wantVal)
			}
			if out.Unit != tc.wantUnit {
				t.Errorf("unit = %q, want %q", out.Unit, tc.wantUnit)
			}
			if out.Quality != "good" {
				t.Errorf("quality = %q, want good", out.Quality)
			}
		})
	}
}

func TestTransformer_InvalidReadingRejected(t *testing.T) {
	tf := NewTransformer(nil, nil)
	_, err := tf.Transform(Reading{DeviceID: "", Metric: "temperature", Value: 1})
	if err == nil {
		t.Fatal("expected error for invalid reading, got nil")
	}
}

func TestPipeline_Apply(t *testing.T) {
	now := time.Now().UTC()
	t1 := NewTransformer(map[string]string{"c": "Celsius"}, nil)
	t2 := NewTransformer(map[string]string{"celsius": "Celsius"}, map[string]string{"temperature": "2"})
	p := &Pipeline{Steps: []*Transformer{t1, t2}}
	out, err := p.Apply(Reading{"d", "temperature", 10, "C", now, "mqtt", "good"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatEq(out.Value, 20) { // 10 * 2
		t.Errorf("value = %v, want 20", out.Value)
	}
	if out.Unit != "Celsius" {
		t.Errorf("unit = %q, want Celsius", out.Unit)
	}
}

func TestPipeline_EmptyReturnsError(t *testing.T) {
	p := &Pipeline{}
	_, err := p.Apply(Reading{"d", "temperature", 10, "C", time.Now().UTC(), "x", "good"})
	if err != ErrEmptyPipeline {
		t.Errorf("err = %v, want ErrEmptyPipeline", err)
	}
}

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
