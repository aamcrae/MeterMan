package core

import (
    "fmt"
    "time"
)

type Accum struct {
    updated bool
    value float64
    current float64
    last float64
    midnight float64
}

func NewAccum(cp string) * Accum {
    a := new(Accum)
    n, err := fmt.Sscanf(cp, "%f %f", &a.midnight, &a.last)
    if err != nil {
        fmt.Printf("%d parsed, accum err: %v\n", n, err)
    }
    a.value = a.last
    if *Verbose {
        fmt.Printf("New accum, midnight = %f, last = %f\n", a.midnight, a.last)
    }
    return a
}

func (a *Accum) Update(v float64) {
    if a.last == 0 {
        a.midnight = v
        a.last = v
    }
    a.value = v
    a.updated = true
}

func (a *Accum) Get() float64 {
    return a.value
}

func (a *Accum) PreWrite(t time.Time) {
}

func (a *Accum) PostWrite() {
    a.updated = false
    a.last = a.value
}

func (a *Accum) Updated() bool {
    return a.updated
}

func (a *Accum) Reset() {
    a.midnight = a.value
}

// Create a checkpoint string.
func (a *Accum) Checkpoint() string {
    return fmt.Sprintf("%f %f", a.midnight, a.last)
}

func (a *Accum) Current() float64 {
    return a.value - a.last
}

func (a *Accum) Total() float64 {
    return a.value
}

func (a *Accum) Daily() float64 {
    return a.value - a.midnight
}
