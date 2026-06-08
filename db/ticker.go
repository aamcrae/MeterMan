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
	"sync/atomic"
	"time"
)

// Ticker holds callbacks to be invoked at the specified period (e.g every 5 minutes)
type Ticker struct {
	cancelled atomic.Bool
}

// NewTicker creates and starts a new ticker.
func NewTicker(tick, offset time.Duration, f func(time.Time)) *Ticker {
	t := &Ticker{}
	// Start a goroutine that invokes a callback for each ticker interval.
	go func() {
		for {
			// Calculate the next time an event should be sent, and
			// sleep until then.
			now := time.Now()
			target := now.Add(tick).Add(-offset).Truncate(tick).Add(offset)
			time.Sleep(target.Sub(now))
			if t.cancelled.Load() {
				return
			}
			f(time.Now())
		}
	}()
	return t
}

func (t *Ticker) Cancel() {
	t.cancelled.Store(true)
}
