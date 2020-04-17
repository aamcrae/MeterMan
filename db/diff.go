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

const kThreshold = 500		// Milliseconds minimum update time

// Diff is a value representing a value derived from an accumulator.
// Typical use would be deriving current Kw from KwH accumulators.
type Diff struct {
	value float64
	previous float64
	ts    time.Time
}

func NewDiff(cp string) *Diff {
	d := new(Diff)
	var sec int64
	fmt.Sscanf(cp, "%f %f %d", &d.value, &d.previous, &sec)
	if sec != 0 {
		d.ts = time.Unix(sec, 0)
	}
	return d
}

func (d *Diff) Update(current float64, ts time.Time) {
	t := ts.Sub(d.ts)
	if t.Milliseconds() >= kThreshold {
		d.value = (current - d.previous) / t.Hours()
	}
	d.previous = current
	d.ts = ts
}

func (d *Diff) Midnight() {
}

func (d *Diff) Get() float64 {
	return d.value
}

func (d *Diff) Timestamp() time.Time {
	return d.ts
}

func (d *Diff) Checkpoint() string {
	return fmt.Sprintf("%f %f %d", d.value, d.previous, d.ts.Unix())
}
