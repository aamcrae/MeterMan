package core

import (
	"fmt"
	"log"
	"time"
)

// Acc is a common interface for accumulators.
type Acc interface {
	Element
	Daily() float64 // Return the daily total.
}

// Accum represents an accumulating value i.e a value that continually increases.
type Accum struct {
	value      float64
	midnight   float64 // Value at the start of the day.
	updated    bool
	resettable bool // If set, the value can be reset to a lower value.
}

func NewAccum(cp string, resettable bool) *Accum {
	a := new(Accum)
	n, err := fmt.Sscanf(cp, "%f %f", &a.midnight, &a.value)
	if err != nil {
		fmt.Printf("%d parsed, accum err: %v\n", n, err)
	}
	if a.midnight > a.value {
		a.midnight = a.value
	}
    a.resettable = resettable
	if *Verbose {
		fmt.Printf("New accum, midnight = %f, value = %f\n", a.midnight, a.value)
	}
	return a
}

func (a *Accum) Update(v float64) {
	// Check whether the accumulator has been reset.
	if v < a.value {
		if !a.resettable {
			log.Printf("Non-resettableccumulator going backwards, value = %f, current = %f\n", v, a.value)
			return
		}
		a.midnight = v
	}
	a.value = v
	a.updated = true
}

func (a *Accum) Get() float64 {
	return a.value
}

func (a *Accum) Interval(last time.Time, midnight bool) {
	if midnight {
		a.midnight = a.value
	}
}

func (a *Accum) Updated() bool {
	return a.updated
}

func (a *Accum) ClearUpdate() {
	a.updated = false
}

// Create a checkpoint string.
func (a *Accum) Checkpoint() string {
	return fmt.Sprintf("%f %f", a.midnight, a.value)
}

func (a *Accum) Daily() float64 {
	return a.value - a.midnight
}

// GetAccum returns the named accumulator.
func GetAccum(name string) Acc {
	el, ok := elements[name]
	if !ok {
		return nil
	}
	switch a := el.(type) {
	case *Accum:
		return a
	case *MultiAccum:
		return a
	default:
		return nil
	}
}
