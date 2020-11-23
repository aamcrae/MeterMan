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
//                                  <- MyProducer sends tagged data updates via channel
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
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aamcrae/config"
)

var verbose = flag.Bool("verbose", false, "Verbose tracing")
var checkpointTick = flag.Int("checkpointrate", 1, "Checkpoint interval (in minutes)")
var checkpoint = flag.String("checkpoint", "", "Checkpoint file")
var startHour = flag.Int("starthour", 5, "Start hour for PV (e.g 6)")
var endHour = flag.Int("endhour", 20, "End hour for PV (e.g 19)")

// DB contains the element database.
type DB struct {
	Config *config.Config // Parsed configuration
	In     chan<- Input   // Write-only channel to receive tagged data
	Trace  bool           // If true, provide tracing
	// StartHour and EndHour define the hours of daylight.
	StartHour int
	EndHour   int

	input      chan Input                // Channel for tagged data
	run        chan func()               // Channel for callbacks
	elements   map[string]Element        // Map of tags to elements
	checkpoint map[string]string         // Initial checkpoint data
	tickers    map[time.Duration]*ticker // Map of tickers
	lastDay    int                       // Current day, to check for midnight processing
}

type callback func(time.Time, time.Time)

// ticker holds callbacks to be invoked at the specified period (e.g every 5 minutes)
type ticker struct {
	tick      time.Duration // Interval time
	last      time.Time     // Last time callbacks were invoked.
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
	d.input = make(chan Input, 200)
	d.In = d.input // Exported write-only input channel
	d.run = make(chan func(), 100)
	return d
}

// Start reads the checkpoint data, calls the init functions,
// and then enters a service loop processing the tag data inputs and tick events.
func (d *DB) Start() error {
	err := d.readCheckpoint()
	if err != nil {
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
	// Start the tickers.
	ec := make(chan event, 10)
	for _, t := range d.tickers {
		t.Start(ec, last)
	}
	for {
		select {
		case r := <-d.input:
			// Received tagged data from producer.
			h, ok := d.elements[r.Tag]
			if ok {
				h.Update(r.Value, time.Now())
			} else {
				log.Printf("Unknown tag: %s\n", r.Tag)
			}
		case ev := <-ec:
			// Event from ticker
			d.tick_event(ev)
		case f := <-d.run:
			// Request to run callback.
			f()
		}
	}
}

// AddCallback adds a callback to be invoked at the interval specified.
func (d *DB) AddCallback(tick time.Duration, cb callback) {
	t, ok := d.tickers[tick]
	if !ok {
		t = new(ticker)
		t.tick = tick
		d.tickers[tick] = t
	}
	t.callbacks = append(t.callbacks, cb)
}

// Execute runs a function in the database thread
func (d *DB) Execute(f func()) {
	var l sync.WaitGroup
	l.Add(1)
	d.run <- func() {
		f()
		l.Done()
	}
	l.Wait()
}

// AddSubGauge adds a sub-gauge to a master gauge.
// If average is true, values are averaged, otherwise they are summed.
// The tag of the new gauge is returned.
func (d *DB) AddSubGauge(base string, average bool) string {
	el, ok := d.elements[base]
	if !ok {
		el = NewMultiElement(base, average)
		d.elements[base] = el
	}
	m := el.(*MultiElement)
	tag := m.NextTag()
	g := NewGauge(d.checkpoint[tag])
	m.Add(g)
	d.elements[tag] = g
	if d.Trace {
		log.Printf("Adding subgauge %s to %s\n", tag, base)
	}
	return tag
}

// AddSubDiff adds a sub-diff to a holding element.
// If average is true, values are averaged, otherwise they are summed.
// The tag of the new Diff is returned.
func (d *DB) AddSubDiff(base string, average bool) string {
	el, ok := d.elements[base]
	if !ok {
		el = NewMultiElement(base, average)
		d.elements[base] = el
	}
	m := el.(*MultiElement)
	tag := m.NextTag()
	nd := NewDiff(d.checkpoint[tag], time.Minute*5)
	m.Add(nd)
	d.elements[tag] = nd
	if d.Trace {
		log.Printf("Adding subdiff %s to %s\n", tag, base)
	}
	return tag
}

// AddSubAccum adds an sub-accumulator to a master accumulator.
// The tag of the new accumulator is returned.
func (d *DB) AddSubAccum(base string, resettable bool) string {
	el, ok := d.elements[base]
	if !ok {
		// Create a new base and add it to the database.
		el = NewMultiAccum(base)
		d.elements[base] = el
	}
	m := el.(*MultiAccum)
	tag := m.NextTag()
	a := NewAccum(d.checkpoint[tag], resettable)
	m.Add(a)
	d.elements[tag] = a
	if d.Trace {
		log.Printf("Adding subaccumulator %s to %s\n", tag, base)
	}
	return tag
}

// AddGauge adds a new gauge to the database.
func (d *DB) AddGauge(name string) {
	d.elements[name] = NewGauge(d.checkpoint[name])
	if d.Trace {
		log.Printf("Adding gauge %s\n", name)
	}
}

// AddDiff adds a new Diff element to the database.
func (d *DB) AddDiff(name string) {
	d.elements[name] = NewDiff(d.checkpoint[name], time.Minute*5)
	if d.Trace {
		log.Printf("Adding diff %s\n", name)
	}
}

// AddAccum adds a new accumulator to the database.
func (d *DB) AddAccum(name string, resettable bool) {
	d.elements[name] = NewAccum(d.checkpoint[name], resettable)
	if d.Trace {
		log.Printf("Adding accumulator %s\n", name)
	}
}

// GetElement returns the named element.
func (d *DB) GetElement(name string) Element {
	return d.elements[name]
}

// GetElements returns the map of elements.
func (d *DB) GetElements() map[string]Element {
	return d.elements
}

// GetAccum returns the named accumulator.
func (d *DB) GetAccum(name string) Acc {
	el, ok := d.elements[name]
	if !ok {
		return nil
	}
	switch a := el.(type) {
	case *Accum:
		return a
	case *MultiAccum:
		return a
	default:
		log.Printf("Tag %s is not an accumulator", name)
		return nil
	}
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
			log.Printf("Output: Tag: %5s, value: %f, timestamp: %s\n", tag, el.Get(), el.Timestamp().Format(time.UnixDate))
		}
	}
	t.ticked(ev.now)
}

// writeCheckpoint saves the values of the elements in the database to a checkpoint file.
func (d *DB) writeCheckpoint(now time.Time) {
	if len(*checkpoint) == 0 {
		return
	}
	if d.Trace {
		log.Printf("Writing checkpoint data to %s\n", *checkpoint)
	}
	f, err := os.Create(*checkpoint)
	if err != nil {
		log.Printf("Checkpoint file create: %s %v\n", *checkpoint, err)
		return
	}
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	for n, e := range d.elements {
		s := e.Checkpoint()
		if len(s) != 0 {
			fmt.Fprintf(wr, "%s:%s\n", n, s)
		}
	}
	fmt.Fprintf(wr, "%s:%d\n", C_TIME, now.Unix())
}

// Checkpoint reads the checkpoint data into a map.
// The checkpoint file contains lines of the form:
//
//    <tag>:<checkpoint string>
//
// When a new element is created, the tag is used to find the checkpoint string
// to be passed to the element's init function so that the element's value can be restored.
func (d *DB) readCheckpoint() error {
	if len(*checkpoint) == 0 {
		return nil
	}
	// Add a callback to checkpoint the database at the specified interval.
	d.AddCallback(time.Minute*time.Duration(*checkpointTick), func(last time.Time, now time.Time) {
		d.writeCheckpoint(now)
	})
	log.Printf("Reading checkpoint data from %s\n", *checkpoint)
	f, err := os.Open(*checkpoint)
	if err != nil {
		return fmt.Errorf("checkpoint file %s: %v", *checkpoint, err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	lineno := 0
	for {
		lineno++
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("checkpoint read %s: line %d: %v", *checkpoint, lineno, err)
			}
			return nil
		}
		s = strings.TrimSuffix(s, "\n")
		i := strings.IndexRune(s, ':')
		if i > 0 {
			d.checkpoint[s[:i]] = s[i+1:]
			if d.Trace {
				log.Printf("Checkpoint entry %s = %s\n", s[:i], s[i+1:])
			}
		}
	}
}

// Initialise and start the ticker.
func (t *ticker) Start(ec chan<- event, last time.Time) {
	log.Printf("Initialising ticker interval %s", t.tick.String())
	// Initialise the tickers with the previous saved tick.
	t.last = last.Truncate(t.tick)
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
		cb(t.last, now)
	}
	t.last = now
}
