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
	"bytes"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

type DbConfig struct {
	Checkpoint string // Checkpoint file
	Update     int    // Update interval for checkpoint in seconds
	Freshness  int    // Number of minutes before data is considered stale
	Daylight   [2]int // Defines the limits of daylight hours
}

const defaultUpdate = 60
const defaultFreshness = 10
const defaultStartHour = 5 // Default start of earliest daylight
const defaultEndHour = 20  // Default end of latest daylight

var freshness int

// DB contains the element database.
type DB struct {
	Config map[string]*yaml.Decoder // Decoded config
	Trace  bool                     // If true, provide tracing
	Dryrun bool                     // If true, validate only
	// StartHour and EndHour define the limit of daylight hours.
	StartHour int
	EndHour   int

	yaml       []byte                    // YAML config
	input      chan input                // Channel for tagged data
	run        chan func()               // Channel for callbacks
	elements   map[string]Element        // Map of tags to elements
	checkpoint map[string]string         // Initial checkpoint data
	disabled   map[string]struct{}       // Map of disabled features
	tickers    map[time.Duration]*ticker // Map of tickers
	lastDay    int                       // Current day, to check for midnight processing
}

type input struct {
	tag   string  // The name of the tag.
	value float64 // The value.
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
func NewDatabase(conf []byte) *DB {
	d := new(DB)
	d.Config = make(map[string]*yaml.Decoder)
	d.yaml = conf
	d.elements = make(map[string]Element)
	d.checkpoint = make(map[string]string)
	d.tickers = make(map[time.Duration]*ticker)
	d.disabled = make(map[string]struct{})
	d.StartHour = defaultStartHour
	d.EndHour = defaultEndHour
	d.input = make(chan input, 200)
	d.run = make(chan func(), 100)
	return d
}

// Start processes the YAML config into separate sections,
// reads the checkpoint data, calls the init functions,
// and then enters a select loop processing the tag data inputs and tick events.
func (d *DB) Start() error {
	// Generate separate subsections for the YAML configuration
	m := make(map[string]interface{})
	err := yaml.Unmarshal(d.yaml, &m)
	if err != nil {
		return err
	}
	for k, v := range m {
		// If a config section has been disabled, do not
		// save that section.
		_, ok := d.disabled[k]
		if ok {
			log.Printf("Disabling feature %s", k)
			continue
		}
		b, err := yaml.Marshal(v)
		if err != nil {
			return fmt.Errorf("YAML marshal of %s failed: %v", k, err)
		}
		// Create a YAML decoder for each separate subsection of the YAML config file.
		d.Config[k] = yaml.NewDecoder(bytes.NewReader(b))
		d.Config[k].KnownFields(true)
		if d.Trace || d.Dryrun {
			log.Printf("YAML section %s = %v", k, v)
		}
	}
	// If configured, read checkpoint file and set up regular updates.
	var conf DbConfig
	yaml, ok := d.Config["db"]
	if ok {
		err := yaml.Decode(&conf)
		if err != nil {
			return err
		}
	}
	// If configured, override the daylight hour limits
	if conf.Daylight[0] != 0 {
		d.StartHour = conf.Daylight[0]
	}
	if conf.Daylight[1] != 0 {
		d.EndHour = conf.Daylight[1]
	}
	// If configured, override the default checkpoint update interval
	update := defaultUpdate
	if conf.Update != 0 {
		update = conf.Update
	}
	// If a checkpoint file is configured, read it, and set up a
	// regular callback to write it. The checkpoint file must be
	// read before the init hooks are called.
	if len(conf.Checkpoint) != 0 {
		log.Printf("Checkpoint file %s, updated every %d seconds", conf.Checkpoint, update)
		if !d.Dryrun {
			if err := d.readCheckpoint(conf.Checkpoint); err != nil {
				return err
			}
			// Add a callback to checkpoint the database at the specified interval.
			d.AddCallback(time.Second*time.Duration(update), func(now time.Time) {
				d.writeCheckpoint(conf.Checkpoint, now)
			})
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
	// Call the init hooks, which initialises all the registered features.
	for _, h := range initHook {
		if err := h(d); err != nil {
			return err
		}
	}
	if d.Dryrun {
		log.Fatalf("Dry run only, exiting")
	}
	// Register some signal handlers for graceful termination
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	// Start the tickers.
	ec := make(chan event, 10)
	for _, t := range d.tickers {
		t.Start(ec, last)
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
			d.writeCheckpoint(conf.Checkpoint, time.Now())
			log.Fatalf("Signal received, exiting")
		}
	}
}

// Disable will disable the selected feature by erasing any
// configuration related to that feature.
func (d *DB) Disable(feat string) {
	d.disabled[feat] = struct{}{}
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
