package core

import (
    "fmt"
    "time"
)


type Gauge struct {
    value float64
    acc float64
    current float64
    last time.Time
    updated bool
}

func NewGauge(cp string) * Gauge {
    g := new(Gauge)
    fmt.Sscanf(cp, "%f", &g.current)
    g.last = lastUpdate
    return g
}

func (g *Gauge) Update(value float64) {
    now := time.Now()
    g.current = value
    g.acc += now.Sub(g.last).Seconds() * g.current
    g.last = now
    g.updated = true
}

func (g *Gauge) PreWrite(t time.Time) {
    g.acc += t.Sub(g.last).Seconds() * g.current
    g.value = g.acc / interval.Seconds()
    g.acc = 0
    g.last = t
    g.current = 0
}

func (g *Gauge) PostWrite() {
    g.updated = false
}

func (g *Gauge) Get() float64 {
    return g.value
}

func (g *Gauge) Updated() bool {
    return g.updated
}

func (g *Gauge) Reset() {
}

func (g *Gauge) Checkpoint() string {
    return fmt.Sprintf("%f", g.value)
}
