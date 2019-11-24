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
	"log"
)

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
	return a
}

func (a *Accum) Update(v float64) {
	// Check whether the accumulator has been reset.
	if v < a.value {
		if !a.resettable {
			log.Printf("Non-resettable accumulator going backwards, value = %f, current = %f\n", v, a.value)
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

func (a *Accum) Midnight() {
	a.midnight = a.value
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
