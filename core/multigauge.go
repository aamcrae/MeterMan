package core

import (
	"fmt"
	"time"
)

// MultiGauge allows multiple gauges to be treated as a single gauge.
// The values are summed.
type MultiGauge struct {
	name    string
    average bool
	gauges  []*Gauge
}

func NewMultiGauge(base string, average bool) *MultiGauge {
	return &MultiGauge{name: base, average:average}
}

func (m *MultiGauge) NextTag() string {
	return fmt.Sprintf("%s/%d", m.name, len(m.gauges))
}

func (m *MultiGauge) Add(g *Gauge) {
	m.gauges = append(m.gauges, g)
}

func (m *MultiGauge) Update(value float64) {
	// Should never happen.
	panic(fmt.Errorf("Updated called on MultiGauge"))
}

func (m *MultiGauge) Interval(t time.Time, midnight bool) {
	for _, g := range m.gauges {
		g.Interval(t, midnight)
	}
}

func (m *MultiGauge) Get() float64 {
	var v float64
	for _, g := range m.gauges {
		v += g.Get()
	}
    if m.average {
        v = v / float64(len(m.gauges))
    }
	return v
}

// Return true only if all sub elements have been updated.
func (m *MultiGauge) Updated() bool {
	for _, g := range m.gauges {
		if !g.Updated() {
			return false
		}
	}
	return true
}

func (m *MultiGauge) ClearUpdate() {
	for _, a := range m.gauges {
		a.ClearUpdate()
	}
}

func (m *MultiGauge) Checkpoint() string {
	return ""
}
