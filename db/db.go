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
	"sync"
	"syscall"
	"time"

	"github.com/aamcrae/MeterMan/lib"
	"gopkg.in/yaml.v3"
)

type DbConfig struct {
	Checkpoint string // Checkpoint file
	Update     int    // Update interval for checkpoint in seconds
	Freshness  int    // Number of minutes before data is considered stale
	Daylight   [2]int // Defines the limits of daylight hours
}

type statusPrinter func() string

const defaultCheckpoint = 60 // Default time between checkpoints (seconds)
const defaultStartHour = 5 // Default start of earliest daylight
const defaultEndHour = 20  // Default end of latest daylight

var freshness int = 10 // Number of minutes before data is considered stale

// DB contains the element database.
type DB struct {
	Config map[string]*yaml.Decoder // Decoded config
	Trace  bool                     // If true, provide tracing
	Dryrun bool                     // If true, validate only
	// StartHour and EndHour define the limit of daylight hours.
	StartHour int
	EndHour   int

	yaml       []byte                        // YAML config
	input      chan input                    // Channel for tagged data
	run        chan func()                   // Channel for callbacks
	elements   map[string]Element            // Map of tags to elements
	checkpoint map[string]string             // Initial checkpoint data
	disabled   map[string]struct{}           // Map of disabled features
	tickers    map[time.Duration]*lib.Ticker // Map of tickers
	lastDay    int                           // Current day, to check for midnight processing
	status	   map[string]statusPrinter		 // Map of status reporters
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
	d.tickers = make(map[time.Duration]*lib.Ticker)
	d.disabled = make(map[string]struct{})
	d.status = make(map[string]statusPrinter)
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
	d.StartHour = lib.ConfigOrDefault(conf.Daylight[0], d.StartHour)
	d.EndHour = lib.ConfigOrDefault(conf.Daylight[1], d.EndHour)
	// If configured, override the freshness timeout
	// If configured, override the default checkpoint update interval
	update := lib.ConfigOrDefault(conf.Update, defaultCheckpoint)
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
	log.Printf("Freshness timeout = %d minutes, daylight start %d:00, end %d:00", freshness, d.StartHour, d.EndHour)
	if d.Dryrun {
		log.Fatalf("Dry run only, exiting")
	}
	if d.Trace {
		// Add a callback dump the state of the database every minute
		d.AddCallback(time.Minute*time.Duration(1), func(now time.Time) {
			d.dumpDB()
		})
	}
	// Register some signal handlers for graceful termination
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	// Start the tickers.
	ec := make(chan lib.Event, 10)
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
			// Event from ticker, run the callbacks in this thread
			ev.Dispatch()
		case f := <-d.run:
			// Request to run callback in main thread
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
func (d *DB) AddCallback(tick time.Duration, cb lib.Callback) {
	t, ok := d.tickers[tick]
	if !ok {
		t = lib.NewTicker(tick)
		if d.Trace {
			t.AddCB(func(now time.Time) {
				log.Printf("Ticker triggered at %s for interval %s\n", now.Format("15:04"), t.Tick().String())
			})
		}
		// Add an initial callback to test for day reset before
		// any other callback runs
		t.AddCB(func(now time.Time) {
			d.checkForMidnight(now, t)
		})
		d.tickers[tick] = t
	}
	t.AddCB(cb)
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

// AddStatusPrinter adds a callback to return status of a feature
func (d *DB) AddStatusPrinter(key string, cb func() string) {
	d.status[key] = cb
}

// Must be called from the main thread
func (d *DB) GetStatus() map[string]string {
	m := make(map[string]string)
	for k, v := range d.status {
		m[k] = v()
	}
	return m
}

// checkForMidnight is the first callback for every ticker.
// - If a new day, call Midnight() on all the database elements.
// - Invoke the update functions.
// It is called from the main select loop.
// Since it cannot be guaranteed which ticker will fire first, this common
// processing will always check for midnight and run the new-day
// processing before anything else.
func (d *DB) checkForMidnight(now time.Time, t *lib.Ticker) {
	// Check for daily reset processing which occurs at midnight.
	atMidnight := now.YearDay() != d.lastDay
	if atMidnight {
		d.lastDay = now.YearDay()
		for _, el := range d.elements {
			el.Midnight()
		}
		if d.Trace {
			log.Printf("Day reset!")
		}
	}
}

// dumpDB Dumps the current state of the database.
func (d *DB) dumpDB() {
	for tag, el := range d.elements {
		log.Printf("Tag: %5s, value: %g, timestamp: %s\n", tag, el.Get(), el.Timestamp().Format(time.UnixDate))
	}
}
