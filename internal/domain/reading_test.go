package domain

import (
	"math"
	"testing"
	"time"
)

func TestReading_Validate(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name    string
		r       Reading
		wantErr bool
	}{
		{"valid", Reading{"dev-1", "temperature", 50, "C", now, "mqtt", "good"}, false},
		{"empty device", Reading{"", "temperature", 50, "C", now, "mqtt", "good"}, true},
		{"empty metric", Reading{"dev-1", "", 50, "C", now, "mqtt", "good"}, true},
		{"NaN value", Reading{"dev-1", "temperature", math.NaN(), "C", now, "mqtt", "good"}, true},
		{"Inf value", Reading{"dev-1", "temperature", math.Inf(1), "C", now, "mqtt", "good"}, true},
		{"zero timestamp", Reading{"dev-1", "temperature", 50, "C", time.Time{}, "mqtt", "good"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
