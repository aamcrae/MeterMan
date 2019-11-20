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

package core

import (
	"fmt"
)

// MultiAccum allows multiple accumulators to be treated as a single accumulator.
// The sub-values are summed.
type MultiAccum struct {
	name   string
	accums []Acc
}

func NewMultiAccum(base string) *MultiAccum {
	return &MultiAccum{name: base}
}

func (m *MultiAccum) NextTag() string {
	return fmt.Sprintf("%s/%d", m.name, len(m.accums))
}

func (m *MultiAccum) Add(a Acc) {
	m.accums = append(m.accums, a)
}

func (m *MultiAccum) Update(v float64) {
	// No one should be updating a multi-accumulator.
	panic(fmt.Errorf("Updated called on MultiAccum"))
}

func (m *MultiAccum) Get() float64 {
	var v float64
	for _, a := range m.accums {
		v += a.Get()
	}
	return v
}

func (m *MultiAccum) Midnight() {
	for _, a := range m.accums {
		a.Midnight()
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

func (m *MultiAccum) Daily() float64 {
	var v float64
	for _, a := range m.accums {
		v += a.Daily()
	}
	return v
}
