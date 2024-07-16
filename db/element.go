// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"time"
)

// Element represents a data item in the database.
type Element interface {
	Update(float64, time.Time) // Update element with new value.
	Midnight()                 // Called when it is midnight for end-of-day processing
	Get() float64              // Get the element's value
	Timestamp() time.Time      // Return the timestamp of the last update.
	Fresh() bool               // Value is still fresh.
	Checkpoint() string        // Return a checkpoint string.
}

// Acc is a common interface for accumulators.
type Acc interface {
	Element
	Daily() float64 // Return the daily total.
}

// AddSubGauge adds a sub-gauge to a master gauge.
// If average is true, values are averaged, otherwise they are summed.
// The tag of the new gauge is returned.
func (d *DB) AddSubGauge(base string, average bool) string {
	el, ok := d.elements[base]
	if !ok {
		el = NewMultiElement(base, average)
		d.elements[base] = el
	}
	m := el.(*MultiElement)
	tag := m.NextTag()
	g := NewGauge(d.checkpoint[tag], d.freshness)
	m.Add(g)
	d.elements[tag] = g
	return tag
}

// AddSubDiff adds a sub-diff to a holding element.
// If average is true, values are averaged, otherwise they are summed.
// The tag of the new Diff is returned.
func (d *DB) AddSubDiff(base string, average bool) string {
	el, ok := d.elements[base]
	if !ok {
		el = NewMultiElement(base, average)
		d.elements[base] = el
	}
	m := el.(*MultiElement)
	tag := m.NextTag()
	nd := NewDiff(d.checkpoint[tag], d.freshness)
	m.Add(nd)
	d.elements[tag] = nd
	return tag
}

// AddSubAccum adds an sub-accumulator to a master accumulator.
// The tag of the new accumulator is returned.
func (d *DB) AddSubAccum(base string, resettable bool) string {
	el, ok := d.elements[base]
	if !ok {
		// Create a new base and add it to the database.
		el = NewMultiAccum(base)
		d.elements[base] = el
	}
	m := el.(*MultiAccum)
	tag := m.NextTag()
	a := NewAccum(d.checkpoint[tag], resettable, d.freshness)
	m.Add(a)
	d.elements[tag] = a
	return tag
}

// AddGauge adds a new gauge to the database.
func (d *DB) AddGauge(name string) {
	d.elements[name] = NewGauge(d.checkpoint[name], d.freshness)
}

// AddDiff adds a new Diff element to the database.
func (d *DB) AddDiff(name string) {
	d.elements[name] = NewDiff(d.checkpoint[name], d.freshness)
}

// AddAccum adds a new accumulator to the database.
func (d *DB) AddAccum(name string, resettable bool) {
	d.elements[name] = NewAccum(d.checkpoint[name], resettable, d.freshness)
}

// GetElement returns the named element.
func (d *DB) GetElement(name string) Element {
	return d.elements[name]
}

// GetElements returns the map of elements.
func (d *DB) GetElements() map[string]Element {
	return d.elements
}

// GetAccum returns the named accumulator.
func (d *DB) GetAccum(name string) Acc {
	el, ok := d.elements[name]
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
