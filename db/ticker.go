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
	"log"
	"time"
)

type callback func(time.Time)

// ticker holds callbacks to be invoked at the specified period (e.g every 5 minutes)
type ticker struct {
	tick      time.Duration // Interval time
	callbacks []callback    // List of callbacks
}

// event is sent from the goroutine when each interval ticks over
type event struct {
	now    time.Time
	ticker *ticker
}

// Initialise and start the ticker.
func (t *ticker) Start(ec chan<- event, last time.Time) {
	log.Printf("Initialising ticker interval %s", t.tick.String())
	// Initialise the tickers with the previous saved tick.
	// Start goroutines that send events for each ticker interval.
	go func(ec chan<- event, t *ticker) {
		var tv event
		tv.ticker = t
		for {
			// Calculate the next time an event should be sent, and
			// sleep until then.
			now := time.Now()
			tv.now = now.Add(t.tick).Truncate(t.tick)
			time.Sleep(tv.now.Sub(now))
			ec <- tv
		}
	}(ec, t)
}

// ticked handles a tick event by invoking the callbacks registered on this ticker.
func (t *ticker) ticked(now time.Time) {
	for _, cb := range t.callbacks {
		cb(now)
	}
}
