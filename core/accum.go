package core

import (
    "fmt"
    "time"
)

type Accum struct {
    value float64
    midnight float64
    lastUpdated bool
    updated bool
}

func NewAccum(cp string) * Accum {
    a := new(Accum)
    n, err := fmt.Sscanf(cp, "%f %f", &a.midnight, &a.value)
    if err != nil {
        fmt.Printf("%d parsed, accum err: %v\n", n, err)
    }
    if *Verbose {
        fmt.Printf("New accum, midnight = %f, value = %f\n", a.midnight, a.value)
    }
    return a
}

func (a *Accum) Update(v float64) {
    // Check whether the accumulator has been reset.
    if v < a.value {
        a.midnight = v
    }
    a.value = v
    a.updated = true
}

func (a *Accum) Get() float64 {
    return a.value
}

func (a *Accum) PreWrite(t time.Time) {
    a.lastUpdated = a.updated
    a.updated = false
}

func (a *Accum) Updated() bool {
    return a.lastUpdated
}

func (a *Accum) Reset() {
    a.midnight = a.value
}

// Create a checkpoint string.
func (a *Accum) Checkpoint() string {
    return fmt.Sprintf("%f %f", a.midnight, a.value)
}

func (a *Accum) Total() float64 {
    return a.value
}

func (a *Accum) Daily() float64 {
    return a.value - a.midnight
}
