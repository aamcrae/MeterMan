package core

import (
	"fmt"
	"time"
)

type Average struct {
	value   float64
	acc     float64
	current float64
	last    time.Time
	updated bool
}

func NewAverage(cp string) *Average {
	g := new(Average)
	fmt.Sscanf(cp, "%f", &g.current)
	g.last = time.Now()
	g.value = g.current
	return g
}

func (g *Average) Update(value float64) {
	g.current = value
	g.acc += time.Now().Sub(g.last).Seconds() * g.current
	g.updated = true
}

func (g *Average) Interval(t time.Time, midnight bool) {
	g.acc += t.Sub(g.last).Seconds() * g.current
	g.value = g.acc / interval.Seconds()
	g.acc = 0
	g.last = t
}

func (g *Average) Get() float64 {
	return g.value
}

func (g *Average) Updated() bool {
	return g.updated
}

func (g *Average) ClearUpdate() {
	g.updated = false
}

func (g *Average) Checkpoint() string {
	return fmt.Sprintf("%f", g.value)
}
