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

// package db stores data sent over a channel from data providers and
// at the selected update interval, invokes handlers to process the data.
// Data can be stored as gauges or accumulators.
// The stored data is checkpointed each update interval.

package db

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aamcrae/config"
)

var verbose = flag.Bool("verbose", false, "Verbose tracing")
var checkpointIntv = flag.Int("checkpointrate", 1, "Checkpoint interval (in minutes)")
var checkpoint = flag.String("checkpoint", "", "Checkpoint file")
var startHour = flag.Int("starthour", 5, "Start hour for PV (e.g 6)")
var endHour = flag.Int("endhour", 20, "End hour for PV (e.g 19)")

// Interval update interface.
type Update interface {
	Update(time.Time, time.Time)
}

type interval struct {
	intv       time.Duration
	last       time.Time
	updateList []Update
}

// DB contains the element database.
type DB struct {
	Trace     bool
	Config    *config.Config
	InChan    chan<- Input
	StartHour int
	EndHour   int

	input         chan Input
	elements      map[string]Element
	outputs       []func(*DB, time.Time)
	intv          time.Duration
	checkpointMap map[string]string
	intvMap       map[time.Duration]*interval
	lastDay       int
}

// tickVal is sent from the ticker goroutine for each interval.
type tickVal struct {
	tick time.Time
	ip   *interval
}

// List of init functions to call after database is ready.
var initHook []func(*DB) error

// Register an init function.
func RegisterInit(f func(*DB) error) {
	initHook = append(initHook, f)
}

// NewDatabase creates a new database.
func NewDatabase(conf *config.Config) *DB {
	d := new(DB)
	d.Trace = *verbose
	d.Config = conf
	d.elements = make(map[string]Element)
	d.checkpointMap = make(map[string]string)
	d.intvMap = make(map[time.Duration]*interval)
	d.StartHour = *startHour
	d.EndHour = *endHour
	d.input = make(chan Input, 200)
	d.InChan = d.input
	return d
}

// Start calls the init functions, and then enters a service loop processing the inputs.
func (d *DB) Start() error {
	err := d.Checkpoint()
	if err != nil {
		return err
	}
	for _, h := range initHook {
		if err := h(d); err != nil {
			return err
		}
	}
	// Get the last time from the checkpoint file.
	var last time.Time
	lt, ok := d.checkpointMap[C_TIME]
	if !ok {
		last = time.Now().Truncate(d.intv)
	} else {
		var sec int64
		fmt.Sscanf(lt, "%d", &sec)
		last = time.Unix(sec, 0)
		if d.Trace {
			log.Printf("Last interval was %s\n", last.Format(time.UnixDate))
		}
	}
	d.lastDay = last.YearDay()
	// Set the last time in the interval map entries.
	for _, ip := range d.intvMap {
		ip.last = last.Truncate(ip.intv)
	}
	tick := make(chan tickVal, 10)
	// Start goroutines to send a tick every intv duration.
	for _, ip := range d.intvMap {
		log.Printf("Initialising interval %s", ip.intv.String())
		go func(ic chan<- tickVal, ip *interval) {
			var t tickVal
			t.ip = ip
			for {
				now := time.Now()
				t.tick = now.Add(ip.intv).Truncate(ip.intv)
				time.Sleep(t.tick.Sub(now))
				ic <- t
			}
		}(tick, ip)
	}
	for {
		select {
		case r := <-d.input:
			// Input from reader.
			h, ok := d.elements[r.Tag]
			if ok {
				h.Update(r.Value, time.Now())
			} else {
				log.Printf("Unknown tag: %s\n", r.Tag)
			}
		case tVal := <-tick:
			d.interval(tVal)
		}
	}
}

// AddUpdate adds a callback to be invoked during interval processing.
func (d *DB) AddUpdate(upd Update, intv time.Duration) {
	ip, ok := d.intvMap[intv]
	if !ok {
		ip = new(interval)
		ip.intv = intv
		d.intvMap[intv] = ip
	}
	ip.updateList = append(ip.updateList, upd)
}

// AddSubGauge adds a sub-gauge to a master gauge.
// If average is true, values are averaged, otherwise they are summed.
// The tag of the new gauge is returned.
func (d *DB) AddSubGauge(base string, average bool) string {
	el, ok := d.elements[base]
	if !ok {
		el = NewMultiGauge(base, average)
		d.elements[base] = el
	}
	m := el.(*MultiGauge)
	tag := m.NextTag()
	g := NewGauge(d.checkpointMap[tag])
	m.Add(g)
	d.elements[tag] = g
	if d.Trace {
		log.Printf("Adding subgauge %s to %s\n", tag, base)
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
	a := NewAccum(d.checkpointMap[tag], resettable)
	m.Add(a)
	d.elements[tag] = a
	if d.Trace {
		log.Printf("Adding subaccumulator %s to %s\n", tag, base)
	}
	return tag
}

// AddGauge adds a new gauge to the database.
func (d *DB) AddGauge(name string) {
	d.elements[name] = NewGauge(d.checkpointMap[name])
	if d.Trace {
		log.Printf("Adding gauge %s\n", name)
	}
}

// AddDiff adds a new Diff element to the database.
func (d *DB) AddDiff(name string) {
	d.elements[name] = NewDiff(d.checkpointMap[name], time.Minute*5)
	if d.Trace {
		log.Printf("Adding diff %s\n", name)
	}
}

// AddAccum adds a new accumulator to the database.
func (d *DB) AddAccum(name string, resettable bool) {
	d.elements[name] = NewAccum(d.checkpointMap[name], resettable)
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

// interval performs the per-interval actions in the following order:
// - If a new day, call Midnight() on all the elements.
// - Invoke the update functions.
func (d *DB) interval(tVal tickVal) {
	ip := tVal.ip
	// Check for daily reset processing.
	midnight := tVal.tick.YearDay() != d.lastDay
	if midnight {
		d.lastDay = tVal.tick.YearDay()
		for _, el := range d.elements {
			el.Midnight()
		}
	}
	if d.Trace {
		log.Printf("Updating at %s for interval %s\n", tVal.tick.Format("15:04"), ip.intv.String())
		if midnight {
			log.Printf("New day reset")
		}
		for tag, el := range d.elements {
			log.Printf("Output: Tag: %5s, value: %f, timestamp: %s\n", tag, el.Get(), el.Timestamp().Format(time.UnixDate))
		}
	}
	for _, u := range ip.updateList {
		u.Update(ip.last, tVal.tick)
	}
	ip.last = tVal.tick
}

// Update will save the current values of the elements in the
// database to a file.
func (d *DB) Update(last, now time.Time) {
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

// Checkpoint reads the checkpoint database.
func (d *DB) Checkpoint() error {
	if len(*checkpoint) == 0 {
		return nil
	}
	d.AddUpdate(d, time.Minute*time.Duration(*checkpointIntv))
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
			d.checkpointMap[s[:i]] = s[i+1:]
			if d.Trace {
				log.Printf("Checkpoint entry %s = %s\n", s[:i], s[i+1:])
			}
		}
	}
}
