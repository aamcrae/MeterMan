package core

import (
	"fmt"
	"time"
)

// Gauge is a value representing a instantaneuous measurement.
type Gauge struct {
	value   float64
	updated bool
}

func NewGauge(cp string) *Gauge {
	g := new(Gauge)
	fmt.Sscanf(cp, "%f", &g.value)
	return g
}

func (g *Gauge) Update(value float64) {
	g.value = value
	g.updated = true
}

func (g *Gauge) Interval(last time.Time, midnight bool) {
}

func (g *Gauge) Get() float64 {
	return g.value
}

func (g *Gauge) Updated() bool {
	return g.updated
}

func (g *Gauge) ClearUpdate() {
	g.updated = false
}

func (g *Gauge) Checkpoint() string {
	return fmt.Sprintf("%f", g.value)
}
