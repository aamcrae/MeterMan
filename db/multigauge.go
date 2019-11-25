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

// MultiGauge allows multiple gauges to be treated as a single gauge.
// The values are summed or averaged depending on the flag.
type MultiGauge struct {
	name    string
	average bool
	gauges  []*Gauge
}

func NewMultiGauge(base string, average bool) *MultiGauge {
	return &MultiGauge{name: base, average: average}
}

func (m *MultiGauge) NextTag() string {
	return fmt.Sprintf("%s/%d", m.name, len(m.gauges))
}

func (m *MultiGauge) Add(g *Gauge) {
	m.gauges = append(m.gauges, g)
}

func (m *MultiGauge) Update(value float64, ts time.Time) {
	// Should never happen.
	panic(fmt.Errorf("Update called on MultiGauge"))
}

func (m *MultiGauge) Midnight() {
	for _, g := range m.gauges {
		g.Midnight()
	}
}

func (m *MultiGauge) Get() float64 {
	var v float64
	for _, g := range m.gauges {
		v += g.Get()
	}
	if m.average {
		v = v / float64(len(m.gauges))
	}
	return v
}

// Return the oldest timestamp.
func (m *MultiGauge) Timestamp() time.Time {
	var timestamp time.Time
	for _, g := range m.gauges {
		ts := g.Timestamp()
		if timestamp.IsZero() || timestamp.After(ts) {
			timestamp = ts
		}
	}
	return timestamp
}

func (m *MultiGauge) Checkpoint() string {
	return ""
}
