package domain

// Thresholds per metric: (warn, critical). Mirrors industrial norms.
var defaultThresholds = map[string][2]float64{
	"temperature": {75, 95},
	"vibration":  {5, 8},
	"pressure":   {8, 12},
	"current":    {40, 60},
	"rpm":        {2500, 3000},
}

// Classify returns NORMAL / WARNING / CRITICAL for a reading using the
// default threshold table. Unknown metrics are NORMAL.
func Classify(r Reading) string {
	limits, ok := defaultThresholds[r.Metric]
	if !ok {
		return "NORMAL"
	}
	switch {
	case r.Value >= limits[1]:
		return "CRITICAL"
	case r.Value >= limits[0]:
		return "WARNING"
	default:
		return "NORMAL"
	}
}
