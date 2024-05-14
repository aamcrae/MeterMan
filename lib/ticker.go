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
	"log"
	"time"
)

// The callbacks are passed the time the ticker ticked over
type Callback func(time.Time)

// ticker holds callbacks to be invoked at the specified period (e.g every 5 minutes)
type Ticker struct {
	tick      time.Duration // Interval duration
	callbacks []Callback    // List of callbacks
}

// Event is sent from the goroutine when each interval ticks over
type Event struct {
	Now    time.Time
	Ticker *Ticker
}

// NewTicker creates and initialises a new ticker
func NewTicker(tick time.Duration) *Ticker {
	return &Ticker{tick: tick}
}

// Initialise and start the ticker.
func (t *Ticker) Start(ec chan<- Event, last time.Time) {
	log.Printf("Initialising ticker interval %s", t.tick.String())
	// Initialise the tickers with the previous saved tick.
	// Start goroutines that send events for each ticker interval.
	go func(ec chan<- Event, t *Ticker) {
		var tv Event
		tv.Ticker = t
		for {
			// Calculate the next time an event should be sent, and
			// sleep until then.
			now := time.Now()
			tv.Now = now.Add(t.tick).Truncate(t.tick)
			time.Sleep(tv.Now.Sub(now))
			ec <- tv
		}
	}(ec, t)
}

// AddCB adds a callback to this ticker's callbacks
func (t *Ticker) AddCB(cb Callback) {
	t.callbacks = append(t.callbacks, cb)
}

// Tick returns the interval duration for this ticker.
func (t *Ticker) Tick() time.Duration {
	return t.tick
}

// Dispatch handles a tick event by invoking the callbacks registered on the ticker.
func (e *Event) Dispatch() {
	for _, cb := range e.Ticker.callbacks {
		cb(e.Now)
	}
}
