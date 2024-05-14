// Copyright 2020 Google LLC
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

type diffValue struct {
	value float64
	ts    time.Time
}

// Diff is a value representing a value derived from an accumulator.
// Typical use would be deriving current Kw from KwH accumulators.
// The window duration defines how long to hold the values before calculating
// the difference.
type Diff struct {
	value    float64 // Current calculated value
	window   time.Duration
	previous []diffValue
	stale    time.Duration // Duration until stale
}

func NewDiff(cp string, window, shelfLife time.Duration) *Diff {
	d := new(Diff)
	d.window = window
	d.stale = shelfLife
	var p diffValue
	var sec int64
	fmt.Sscanf(cp, "%f %f %d", &d.value, &p.value, &sec)
	if sec != 0 {
		p.ts = time.Unix(sec, 0)
	}
	d.previous = append(d.previous, p)
	return d
}

func (d *Diff) Update(current float64, ts time.Time) {
	t := ts.Add(-d.window)
	// Remove elements that are outside the time window.
	for len(d.previous) > 0 && !d.previous[0].ts.After(t) {
		d.previous = d.previous[1:]
	}
	d.previous = append(d.previous, diffValue{current, ts})
	// Calculate value if there are at least 2 items.
	if len(d.previous) >= 2 {
		td := ts.Sub(d.previous[0].ts)
		d.value = (current - d.previous[0].value) / td.Hours()
	}
}

func (d *Diff) Midnight() {
}

func (d *Diff) Get() float64 {
	return d.value
}

func (d *Diff) Timestamp() time.Time {
	return d.previous[len(d.previous)-1].ts
}

func (d *Diff) Fresh() bool {
	return !d.Timestamp().Before(time.Now().Add(-d.stale))
}

func (d *Diff) Checkpoint() string {
	return fmt.Sprintf("%g %g %d", d.value, d.previous[0].value, d.previous[0].ts.Unix())
}
