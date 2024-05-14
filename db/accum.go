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
	"time"
)

// Accum represents an accumulating value i.e a value that continually increases.
type Accum struct {
	value      float64
	midnight   float64       // Value at the start of the day.
	resettable bool          // If set, the value can be reset to a lower value.
	ts         time.Time     // Timestamp of last update.
	stale      time.Duration // Duration until stale
}

func NewAccum(cp string, resettable bool, shelfLife time.Duration) *Accum {
	a := new(Accum)
	a.stale = shelfLife
	if len(cp) != 0 {
		var sec int64
		n, err := fmt.Sscanf(cp, "%f %f %d", &a.midnight, &a.value, &sec)
		if sec != 0 {
			a.ts = time.Unix(sec, 0)
		}
		if err != nil {
			fmt.Printf("%d parsed, accum err: %v\n", n, err)
		}
	}
	if a.midnight > a.value {
		a.midnight = a.value
	}
	a.resettable = resettable
	return a
}

func (a *Accum) Update(v float64, ts time.Time) {
	// Check whether the accumulator has been reset.
	if v < a.value {
		if !a.resettable {
			log.Printf("Non-resettable accumulator change ignored, value = %g, current = %g, midnight = %g\n", v, a.value, a.midnight)
			return
		}
		if v < a.midnight {
			log.Printf("Accumulator reset, new value = %g, current = %g, midnight = %g\n", v, a.value, a.midnight)
			a.midnight = v
		} else {
			log.Printf("Accumulator going backwards, ignored, value = %g, current = %g, midnight = %g\n", v, a.value, a.midnight)
		}
	}
	a.value = v
	a.ts = ts
}

func (a *Accum) Get() float64 {
	return a.value
}

func (a *Accum) Midnight() {
	a.midnight = a.value
}

func (a *Accum) Timestamp() time.Time {
	return a.ts
}

func (a *Accum) Fresh() bool {
	return !a.Timestamp().Before(time.Now().Add(-a.stale))
}

// Create a checkpoint string.
func (a *Accum) Checkpoint() string {
	return fmt.Sprintf("%g %g %d", a.midnight, a.value, a.ts.Unix())
}

func (a *Accum) Daily() float64 {
	return a.value - a.midnight
}
