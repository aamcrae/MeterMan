package core

import (
	"fmt"
	"time"
)

type MultiGauge struct {
	name   string
	gauges []*Gauge
}

func NewMultiGauge(base string) *MultiGauge {
	return &MultiGauge{name: base}
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
