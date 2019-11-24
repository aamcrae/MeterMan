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

// package db stores data sent over a channel from data providers ('readers') and
// at the selected update interval, invokes 'writers' to process the data.
// Data can be stored as gauges or accumulators.
// The stored data is checkpointed each update interval.

package db

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aamcrae/config"
)

// DB contains the central database and variables.
type DB struct {
	Trace     bool
	Config    *config.Config
	InChan    chan<- Input
	StartHour int
	EndHour   int

	elements      map[string]Element
	outputs       []func(*DB, time.Time)
	intv          time.Duration
	checkpoint    string
	checkpointMap map[string]string
}

var writersInit []func(*DB) (func(*DB, time.Time), error)
var readersInit []func(*DB) error

// Register a 'writer' i.e a function that accesses the collated data and
// processes it (e.g writes it to a file).
func RegisterWriter(f func(*DB) (func(*DB, time.Time), error)) {
	writersInit = append(writersInit, f)
}

// Register a 'reader', a module that reads outside data and sends to the
// database via the input channel.
func RegisterReader(f func(*DB) error) {
	readersInit = append(readersInit, f)
}

// NewDatabase creates a new database with the updateRate (in minutes)
func NewDatabase(conf *config.Config, updateRate int) *DB {
	d := new(DB)
	d.Config = conf
	d.elements = make(map[string]Element)
	d.checkpointMap = make(map[string]string)
	d.intv = time.Minute * time.Duration(updateRate)
	d.StartHour = 6
	d.EndHour = 20
	return d
}

// Start calls the init functions for the readers and writers,
// and then goes into a service loop processing the inputs from the readers.
func (d *DB) Start() error {
	input := make(chan Input, 200)
	d.InChan = input
	for _, wi := range writersInit {
		if of, err := wi(d); err != nil {
			return err
		} else if of != nil {
			d.outputs = append(d.outputs, of)
		}
	}
	for _, ri := range readersInit {
		if err := ri(d); err != nil {
			return err
		}
	}
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
	tick := ticker(d.intv)
	for {
		select {
		case r := <-input:
			// Input from reader.
			h, ok := d.elements[r.Tag]
			if ok {
				h.Update(r.Value)
			} else {
				log.Printf("Unknown tag: %s\n", r.Tag)
			}
		case now := <-tick:
			// Update tick.
			d.update(last, now)
			last = now
		}
	}
}

// ticker sends time through a channel each intv time.
func ticker(intv time.Duration) <-chan time.Time {
	t := make(chan time.Time, 10)
	go func() {
		for {
			now := time.Now()
			next := now.Add(intv).Truncate(intv)
			time.Sleep(next.Sub(now))
			t <- next
		}
	}()
	return t
}

// AddSumGauge adds a gauge that is part of a master gauge.
// If average is true, values are averaged, otherwise they are summed.
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

// AddSubAccum adds an accumulator that is part of a master accumulator.
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

// update performs the per-interval actions in the following order:
// - If a new day, call Midnight().
// - Invoke the output functions.
// - Write the checkpoint file.
// - Clear the Updated flag.
func (d *DB) update(last, now time.Time) {
	// Check for daily reset processing.
	midnight := now.YearDay() != last.YearDay()
	if midnight {
		for _, el := range d.elements {
			el.Midnight()
		}
	}
	if d.Trace {
		log.Printf("Updating for interval %s\n", now.Format("15:04"))
		if midnight {
			log.Printf("New day reset")
		}
		for tag, el := range d.elements {
			log.Printf("Output: Tag: %5s, value: %f, updated: %v\n", tag, el.Get(), el.Updated())
		}
	}
	for _, wf := range d.outputs {
		wf(d, now)
	}
	d.writeCheckpoint(now)
	for _, el := range d.elements {
		el.ClearUpdate()
	}
}

// writerCheckpoint will save the current values of the elements in the
// database to a file.
func (d *DB) writeCheckpoint(now time.Time) {
	if len(d.checkpoint) == 0 {
		return
	}
	if d.Trace {
		log.Printf("Writing checkpoint data to %s\n", d.checkpoint)
	}
	f, err := os.Create(d.checkpoint)
	if err != nil {
		log.Printf("Checkpoint file create: %s %v\n", d.checkpoint, err)
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

// Checkpoint sets the name of the checkpoint file, and
// does an initial read of the checkpoint database.
func (d *DB) Checkpoint(checkpoint string) error {
	d.checkpoint = checkpoint
	if len(d.checkpoint) == 0 {
		return nil
	}
	log.Printf("Reading checkpoint data from %s\n", d.checkpoint)
	f, err := os.Open(d.checkpoint)
	if err != nil {
		return fmt.Errorf("checkpoint file %s: %v", d.checkpoint, err)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	lineno := 0
	for {
		lineno++
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("checkpoint read %s: line %d: %v", d.checkpoint, lineno, err)
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
