package core

import (
    "fmt"
    "time"
)

type MultiAccum struct {
    name string
    accums []Acc
}

func NewMultiAccum(base string) * MultiAccum {
    return &MultiAccum{name:base}
}

func (m *MultiAccum) NextTag() string {
    return fmt.Sprintf("%s/%d", m.name, len(m.accums))
}

func (m *MultiAccum) Add(a Acc) {
    m.accums = append(m.accums, a)
}

func (m *MultiAccum) Update(v float64) {
    panic(fmt.Errorf("Updated called on MultiAccum"))
}

func (m *MultiAccum) Get() float64 {
    var v float64
    for _, a := range m.accums {
        v += a.Get()
    }
    return v
}

func (m *MultiAccum) Interval(t time.Time, midnight bool) {
    for _, a := range m.accums {
        a.Interval(t, midnight)
    }
}

// Return true only if all sub elements have been updated.
func (m *MultiAccum) Updated() bool {
    for _, a := range m.accums {
        if !a.Updated() {
            return false
        }
    }
    return true
}

func (m *MultiAccum) ClearUpdate() {
    for _, a := range m.accums {
        a.ClearUpdate()
    }
}

func (m *MultiAccum) Checkpoint() string {
    return ""
}

func (m *MultiAccum) Total() float64 {
    var v float64
    for _, a := range m.accums {
        v += a.Total()
    }
    return v
}

func (m *MultiAccum) Daily() float64 {
    var v float64
    for _, a := range m.accums {
        v += a.Daily()
    }
    return v
}
