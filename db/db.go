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

// package db stores tagged data sent over a channel from data providers,
// and at specified intervals, invokes handlers to process the data.
// Data can be stored as gauges or accumulators.
// The stored data is checkpointed at selected intervals.
//
// Data processing uses a provider/consumer model. Providers send
// tagged values via a common input channel. Consumers register themselves
// to be invoked at intervals (such as 1 minute, 5 minutes etc).
//
// Access to the elements database should only be done in the context of the
// thread that calls Start(). This thread processes the received tagged
// data sent via the input channel, updating the database as required.
// Interval callbacks (for consumers) are invoked as part of this same
// thread, so callbacks can freely access the element database - as a result,
// consumer callbacks should not block or delay unnecessarily.
//
// Initialisation is handled as a set-up phase, where modules register an init hook
// (via an init() function) by calling RegisterInit().
//
//                 (MyModule) init()
//                            -> RegisterInit(MyInit)
//  db.Start()
//             -> MyInit(db)
//                   -> Initialise module, save input channel, start producer goroutine
//                   -> db.AddCallback(interval, MyConsumer)  [to register consumer]
//     ...
//  <processing loop>
//                                  <- MyProducer db.Input()
//    <receives tagged data>
//          <updates elements>
//    <each-interval>
//          <invokes interval callbacks>
//                      -> MyConsumer  reads and processes elements
//
// All setup (registering callbacks, creating database elements etc) must be completed
// as part of the Start() initialisation, before the processing loop is entered.

package db

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aamcrae/config"
)

var verbose = flag.Bool("verbose", false, "Verbose tracing")
var dryrun = flag.Bool("dryrun", false, "Validate config only")
var startHour = flag.Int("starthour", 5, "Start hour for PV (e.g 6)")
var endHour = flag.Int("endhour", 20, "End hour for PV (e.g 19)")
var freshness = flag.Int("freshness", 10, "Default minutes until data is stale")

// DB contains the element database.
type DB struct {
	Config *config.Config // Parsed configuration
	Trace  bool           // If true, provide tracing
	Dryrun bool           // If true, validate only
	// StartHour and EndHour define the hours of daylight.
	StartHour int
	EndHour   int

	input      chan input                // Channel for tagged data
	run        chan func()               // Channel for callbacks
	elements   map[string]Element        // Map of tags to elements
	checkpoint map[string]string         // Initial checkpoint data
	tickers    map[time.Duration]*ticker // Map of tickers
	lastDay    int                       // Current day, to check for midnight processing
}

type input struct {
	tag   string  // The name of the tag.
	value float64 // The value.
}

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

// List of functions to call after checkpoint data is available.
// Used to initialise database elements.
var initHook []func(*DB) error

// Register an init function.
// These will be called once the checkpoint data is read.
func RegisterInit(f func(*DB) error) {
	initHook = append(initHook, f)
}

// NewDatabase creates a new database handler.
func NewDatabase(conf *config.Config) *DB {
	d := new(DB)
	d.Trace = *verbose
	d.Config = conf
	d.elements = make(map[string]Element)
	d.checkpoint = make(map[string]string)
	d.tickers = make(map[time.Duration]*ticker)
	d.StartHour = *startHour
	d.EndHour = *endHour
	d.input = make(chan input, 200)
	d.run = make(chan func(), 100)
	return d
}

// Start reads the checkpoint data, calls the init functions,
// and then enters a select loop processing the tag data inputs and tick events.
func (d *DB) Start() error {
	if err := d.readCheckpoint(); err != nil {
		return err
	}
	// Call the init hooks, which initialises all the registered modules.
	for _, h := range initHook {
		if err := h(d); err != nil {
			return err
		}
	}
	// Get the last saved time from the checkpoint file.
	var last time.Time
	lt, ok := d.checkpoint[C_TIME]
	if !ok {
		last = time.Now()
	} else {
		var sec int64
		fmt.Sscanf(lt, "%d", &sec)
		last = time.Unix(sec, 0)
		if d.Trace {
			log.Printf("Last time saved was %s\n", last.Format(time.UnixDate))
		}
	}
	d.lastDay = last.YearDay()
	// Register some signal handlers for graceful termination
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	// Start the tickers.
	ec := make(chan event, 10)
	for _, t := range d.tickers {
		t.Start(ec, last)
	}
	if *dryrun {
		log.Fatalf("Dryrun only, exiting")
	}
	for {
		select {
		case r := <-d.input:
			// Received tagged data from producer.
			h, ok := d.elements[r.tag]
			if ok {
				h.Update(r.value, time.Now())
			} else {
				log.Printf("Unknown tag: %s\n", r.tag)
			}
		case ev := <-ec:
			// Event from ticker
			d.tick_event(ev)
		case f := <-d.run:
			// Request to run callback from main thread
			f()
		case <-sigc:
			// Signal caught, write checkpoint file and exit.
			d.writeCheckpoint(time.Now())
			log.Fatalf("Signal received, exiting")
		}
	}
}

// Input sends tagged input data to the input channel
func (d *DB) Input(tag string, value float64) {
	d.input <- input{tag, value}
}

// AddCallback adds a callback to be regularly invoked at the interval specified.
func (d *DB) AddCallback(tick time.Duration, cb callback) {
	t, ok := d.tickers[tick]
	if !ok {
		t = new(ticker)
		t.tick = tick
		d.tickers[tick] = t
	}
	t.callbacks = append(t.callbacks, cb)
}

// Execute runs a function in the main thread, blocking until
// the function returns.
func (d *DB) Execute(f func()) {
	var l sync.WaitGroup
	l.Add(1)
	d.run <- func() {
		f()
		l.Done()
	}
	l.Wait()
}

// tick_event handles a new ticker event with common processing:
// - If a new day, call Midnight() on all the database elements.
// - Invoke the update functions.
func (d *DB) tick_event(ev event) {
	t := ev.ticker
	// Check for daily reset processing which occurs at midnight.
	midnight := ev.now.YearDay() != d.lastDay
	if midnight {
		d.lastDay = ev.now.YearDay()
		for _, el := range d.elements {
			el.Midnight()
		}
	}
	if d.Trace {
		log.Printf("Updating at %s for interval %s\n", ev.now.Format("15:04"), t.tick.String())
		if midnight {
			log.Printf("New day reset")
		}
		for tag, el := range d.elements {
			log.Printf("Output: Tag: %5s, value: %g, timestamp: %s\n", tag, el.Get(), el.Timestamp().Format(time.UnixDate))
		}
	}
	t.ticked(ev.now)
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

// FmtFloat is a custom float formatter that
// has a fixed precision of 2 decimal places with trailing zeros removed.
func FmtFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	last := len(s) - 1
	if s[last] == '.' {
		s = s[:last]
	}
	return s
}
