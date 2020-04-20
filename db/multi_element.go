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
	"fmt"
	"time"
)

// MultiElement allows multiple elements to be treated as a single elements.
// The values are summed or averaged depending on the flag.
type MultiElement struct {
	name     string
	average  bool
	elements []Element
}

func NewMultiElement(base string, average bool) *MultiElement {
	return &MultiElement{name: base, average: average}
}

func (m *MultiElement) NextTag() string {
	return fmt.Sprintf("%s/%d", m.name, len(m.elements))
}

func (m *MultiElement) Add(g Element) {
	m.elements = append(m.elements, g)
}

func (m *MultiElement) Update(value float64, ts time.Time) {
	// Should never happen.
	panic(fmt.Errorf("Update called on MultiElement"))
}

func (m *MultiElement) Midnight() {
	for _, g := range m.elements {
		g.Midnight()
	}
}

func (m *MultiElement) Get() float64 {
	var v float64
	for _, g := range m.elements {
		v += g.Get()
	}
	if m.average {
		v = v / float64(len(m.elements))
	}
	return v
}

// Return the oldest timestamp.
func (m *MultiElement) Timestamp() time.Time {
	var timestamp time.Time
	for _, g := range m.elements {
		ts := g.Timestamp()
		if timestamp.IsZero() || timestamp.After(ts) {
			timestamp = ts
		}
	}
	return timestamp
}

func (m *MultiElement) Checkpoint() string {
	return ""
}
