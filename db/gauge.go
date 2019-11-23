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
)

// Gauge is a value representing a instantaneous measurement.
// If multiple updates occur during an interval, an average is taken.
type Gauge struct {
	value   float64
	total   float64
	updated int
}

func NewGauge(cp string) *Gauge {
	g := new(Gauge)
	fmt.Sscanf(cp, "%f", &g.value)
	return g
}

func (g *Gauge) Update(value float64) {
	g.total += value
	g.updated++
	g.value = g.total / float64(g.updated)
}

func (g *Gauge) Midnight() {
}

func (g *Gauge) Get() float64 {
	return g.value
}

func (g *Gauge) Updated() bool {
	return g.updated != 0
}

func (g *Gauge) ClearUpdate() {
	g.updated = 0
	g.total = 0
}

func (g *Gauge) Checkpoint() string {
	return fmt.Sprintf("%f", g.value)
}
