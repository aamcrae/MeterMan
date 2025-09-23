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

package lib

import (
	"fmt"
	"strings"
	"time"
)

// Ticker holds callbacks to be invoked at the specified period (e.g every 5 minutes)
type Ticker struct {
	tick      time.Duration     // Interval duration
	offset    time.Duration     // Offset from interval (+ve or -ve)
	next      time.Time         // Target time of next trigger
	fired     int               // Number of times fired
	callbacks []func(time.Time) // List of callbacks
}

// Event is sent from the per-ticker goroutine to a common channel when the ticker interval ticks over
type Event struct {
	target time.Time
	ticker *Ticker
}

// NewTicker creates and initialises a new ticker
func NewTicker(tick, offset time.Duration) *Ticker {
	return &Ticker{tick: tick, offset: offset}
}

// Start initialises and starts the ticker by
// launching a goroutine that waits for the ticker
// interval, and then sends an event on the channel provided.
func (t *Ticker) Start(ec chan<- Event) {
	// Start a goroutine that sends an event for each ticker interval.
	go func(ec chan<- Event, t *Ticker) {
		var tv Event
		tv.ticker = t
		for {
			// Calculate the next time an event should be sent, and
			// sleep until then.
			now := time.Now()
			tv.target = now.Add(t.tick).Add(-t.offset).Truncate(t.tick).Add(t.offset)
			t.next = tv.target
			time.Sleep(tv.target.Sub(now))
			ec <- tv
			t.fired++
		}
	}(ec, t)
}

// AddCB adds a callback to this ticker's callbacks
func (t *Ticker) AddCB(cb func(time.Time)) {
	t.callbacks = append(t.callbacks, cb)
}

// Tick returns the interval duration for this ticker.
func (t *Ticker) Tick() time.Duration {
	return t.tick
}

func (t *Ticker) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "interval %s, offset %s, callbacks %d, fired %d", t.tick, t.offset, len(t.callbacks), t.fired)
	if !t.next.IsZero() {
		fmt.Fprintf(&b, ", next firing %s", t.next)
	}
	return b.String()
}

// Dispatch handles a tick event by invoking the callbacks registered on the ticker.
func (e *Event) Dispatch() {
	for _, cb := range e.ticker.callbacks {
		cb(e.target)
	}
}
