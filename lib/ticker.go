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
	"sync"
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
type event struct {
	target time.Time
	ticker *Ticker
}

// Key for map
type tickKey struct {
	tick time.Duration
	offs time.Duration
}

var waitChan chan event
var evOnce sync.Once
var Tickers map[tickKey]*Ticker = map[tickKey]*Ticker{}

// NewTicker creates and starts (if necessary) a new ticker.
func NewTicker(tick, offset time.Duration) *Ticker {
	key := tickKey{tick, offset}
	t, ok := Tickers[key]
	if !ok {
		// New ticker entry
		t = &Ticker{tick: tick, offset: offset}
		Tickers[key] = t
		// Start a goroutine that sends an event for each ticker interval.
		go func() {
			ec := getChan()
			var tv event
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
		}()
	}
	return t
}

func getChan() chan event {
	evOnce.Do(func() {
		waitChan = make(chan event, 10)
	})
	return waitChan
}

func WaitChan() <-chan event {
	return getChan()
}

// AddCB adds a callback to this ticker's callbacks
func (t *Ticker) AddCB(cb func(time.Time)) {
	t.callbacks = append(t.callbacks, cb)
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
func (e *event) Dispatch() {
	for _, cb := range e.ticker.callbacks {
		cb(e.target)
	}
}
