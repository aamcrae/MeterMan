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

// Diff is a value representing a value derived from an accumulator, based on hours.
// Typical use would be deriving current Kw from KwH accumulators.
type Diff struct {
	value         float64 // Current calculated value
	previousValue float64
	previousTime  time.Time
	stale         time.Duration // Duration until stale
}

func NewDiff(cp string, shelfLife time.Duration) *Diff {
	d := new(Diff)
	d.stale = shelfLife
	var sec int64
	fmt.Sscanf(cp, "%f %f %d", &d.value, &d.previousValue, &sec)
	if sec != 0 {
		d.previousTime = time.Unix(sec, 0)
	}
	return d
}

func (d *Diff) Update(current float64, ts time.Time) {
	// Calculate value if the previous value is valid
	if current >= d.previousValue && !d.previousTime.IsZero() {
		td := ts.Sub(d.previousTime)
		if td.Seconds() > 1 {
			// Skip if samples are too close together.
			d.value = (current - d.previousValue) / td.Hours()
		}
	}
	d.previousValue = current
	d.previousTime = ts
}

func (d *Diff) Midnight() {
}

func (d *Diff) Get() float64 {
	return d.value
}

func (d *Diff) Timestamp() time.Time {
	return d.previousTime
}

func (d *Diff) Fresh() bool {
	return !d.previousTime.Before(time.Now().Add(-d.stale))
}

func (d *Diff) Checkpoint() string {
	return fmt.Sprintf("%g %g %d", d.value, d.previousValue, d.previousTime.Unix())
}
