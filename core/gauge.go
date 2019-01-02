package core

import (
    "fmt"
    "time"
)


type Gauge struct {
    value float64
    last time.Time
    updated time.Time
}

func NewGauge(cp string) * Gauge {
    g := new(Gauge)
    fmt.Sscanf(cp, "%f", &g.value)
    g.last = time.Now()
    return g
}

func (g *Gauge) Update(t time.Time, value float64) {
    g.value = value
    g.updated = t
}

func (g *Gauge) Interval(last time.Time, midnight bool) {
    g.last = last
}

func (g *Gauge) Get() float64 {
    return g.value
}

func (g *Gauge) Updated() bool {
    return g.updated.After(g.last)
}

func (g *Gauge) Checkpoint() string {
    return fmt.Sprintf("%f", g.value)
}
