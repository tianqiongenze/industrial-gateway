package domain

import (
	"errors"
	"fmt"
	"strings"
)

// Transformer applies unit normalisation + optional scaling to a raw reading so
// that every reading leaving the gateway has a canonical shape & unit. Pure
// business logic, fully unit-tested without I/O.
type Transformer struct {
	normaliseUnits map[string]string // raw unit -> canonical unit
	scaling       map[string]float64 // metric -> multiplier (raw->SI)
}

// NewTransformer builds a transformer from config maps.
func NewTransformer(unitMap, scaling map[string]string) *Transformer {
	t := &Transformer{normaliseUnits: map[string]string{}, scaling: map[string]float64{}}
	for k, v := range unitMap {
		t.normaliseUnits[strings.ToLower(k)] = v
	}
	for k, v := range scaling {
		if f, err := parseFloat(v); err == nil {
			t.scaling[strings.ToLower(k)] = f
		}
	}
	return t
}

// Transform applies unit normalisation + optional scaling and marks quality.
func (t *Transformer) Transform(r Reading) (Reading, error) {
	if err := r.Validate(); err != nil {
		return Reading{}, fmt.Errorf("invalid reading: %w", err)
	}
	out := r
	out.Source = r.Source
	out.Quality = "good"
	// Fahrenheit -> Celsius special-case (applied before unit map).
	if strings.EqualFold(r.Unit, "F") {
		out.Value = (r.Value - 32) * 5 / 9
		out.Unit = "Celsius"
	} else if canon, ok := t.normaliseUnits[strings.ToLower(r.Unit)]; ok {
		out.Unit = canon
	}
	// scaling (raw sensor counts -> engineering unit), applied to the value.
	if mult, ok := t.scaling[strings.ToLower(r.Metric)]; ok && mult != 0 {
		out.Value = out.Value * mult
	}
	if out.Unit == "" {
		out.Unit = "unknown"
	}
	return out, nil
}

// Pipeline is an ordered sequence of transformers applied left to right.
type Pipeline struct {
	Steps []*Transformer
}

// Apply runs every transformer in order; the first error short-circuits.
func (p *Pipeline) Apply(r Reading) (Reading, error) {
	if len(p.Steps) == 0 {
		return r, ErrEmptyPipeline
	}
	var err error
	for _, s := range p.Steps {
		if r, err = s.Transform(r); err != nil {
			return Reading{}, err
		}
	}
	return r, nil
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

var ErrEmptyPipeline = errors.New("pipeline has no steps")
