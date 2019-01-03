package core

import (
    "fmt"
    "time"
)

type Acc interface {
    Element
    Total() float64
    Daily() float64
}

type Accum struct {
    value float64
    midnight float64
    last time.Time
    updated time.Time
}

func NewAccum(cp string) * Accum {
    a := new(Accum)
    n, err := fmt.Sscanf(cp, "%f %f", &a.midnight, &a.value)
    if err != nil {
        fmt.Printf("%d parsed, accum err: %v\n", n, err)
    }
    a.last = time.Now()
    if a.midnight > a.value {
        a.midnight = a.value
    }
    if *Verbose {
        fmt.Printf("New accum, midnight = %f, value = %f\n", a.midnight, a.value)
    }
    return a
}

func (a *Accum) Update(now time.Time, v float64) {
    // Check whether the accumulator has been reset.
    if v < a.value {
        a.midnight = v
    }
    a.value = v
    a.updated = now
}

func (a *Accum) Get() float64 {
    return a.value
}

func (a *Accum) Interval(last time.Time, midnight bool) {
    a.last = last
    if midnight {
        a.midnight = a.value
    }
}

func (a *Accum) Updated() bool {
    return a.updated.After(a.last)
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
