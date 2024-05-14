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

// Gauge is a value representing a instantaneous measurement.
// If multiple updates occur during an interval, an average is taken.
type Gauge struct {
	value float64
	ts    time.Time
	stale time.Duration // Duration until stale
}

func NewGauge(cp string) *Gauge {
	g := new(Gauge)
	var sec int64
	fmt.Sscanf(cp, "%f %d", &g.value, &sec)
	if sec != 0 {
		g.ts = time.Unix(sec, 0)
	}
	g.SetFreshness(time.Minute * time.Duration(freshness))
	return g
}

func (g *Gauge) Update(value float64, ts time.Time) {
	g.value = value
	g.ts = ts
}

func (g *Gauge) Midnight() {
}

func (g *Gauge) Get() float64 {
	return g.value
}

func (g *Gauge) Timestamp() time.Time {
	return g.ts
}

func (g *Gauge) SetFreshness(s time.Duration) {
	g.stale = s
}

func (g *Gauge) Fresh() bool {
	return !g.Timestamp().Before(time.Now().Add(-g.stale))
}

func (g *Gauge) Checkpoint() string {
	return fmt.Sprintf("%g %d", g.value, g.ts.Unix())
}
