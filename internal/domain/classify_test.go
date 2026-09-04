package domain

import (
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name   string
		r      Reading
		expect string
	}{
		{"temp normal", Reading{"d", "temperature", 30, "C", now, "x", "good"}, "NORMAL"},
		{"temp warning", Reading{"d", "temperature", 80, "C", now, "x", "good"}, "WARNING"},
		{"temp critical", Reading{"d", "temperature", 96, "C", now, "x", "good"}, "CRITICAL"},
		{"vibration warning", Reading{"d", "vibration", 6, "mm/s", now, "x", "good"}, "WARNING"},
		{"vibration critical", Reading{"d", "vibration", 9, "mm/s", now, "x", "good"}, "CRITICAL"},
		{"unknown metric normal", Reading{"d", "humidity", 999, "%", now, "x", "good"}, "NORMAL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.r)
			if got != tc.expect {
				t.Errorf("Classify() = %q, want %q", got, tc.expect)
			}
		})
	}
}
