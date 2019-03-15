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

// package core stores data sent over a channel from data providers ('readers') and
// at the selected update interval, sends the stored data to 'writers'.
// Data can be stored as gauges or accumulators.
// The stored data is checkpointed each update interval.

package core

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

var Verbose = flag.Bool("verbose", false, "Verbose tracing")
var updateRate = flag.Int("update", 5, "Update rate (in minutes)")
var checkpoint = flag.String("checkpoint", "", "Checkpoint file")

// Input represents the data sent by each reader.
type Input struct {
	Tag   string  // The name of the tag.
	Value float64 // The current value.
}

// Element represents each data item that is being updated by the readers.
type Element interface {
	Update(v float64)                       // Update element with new value.
	Interval(last time.Time, midnight bool) // Called before uploading.
	Get() float64                           // Get the element's value
	Updated() bool                          // Return true if value has been updated in this interval.
	ClearUpdate()                           // Reset the update flag.
	Checkpoint() string                     // Return a checkpoint string.
}

var elements map[string]Element = map[string]Element{}
var checkpointMap map[string]string = make(map[string]string)

var interval time.Duration
var lastInterval time.Time

// Callbacks for processing outputs.
var outputs []func(time.Time)

var writersInit []func(*config.Config) (func(time.Time), error)
var readersInit []func(*config.Config, chan<- Input) error

// Register a 'writer' i.e a function that takes the collated data and
// processes it (e.g writes it to a file).
func RegisterWriter(f func(*config.Config) (func(time.Time), error)) {
	writersInit = append(writersInit, f)
}

// Register a 'reader', a module that reads data and sends it via the
// provided channel.
func RegisterReader(f func(*config.Config, chan<- Input) error) {
	readersInit = append(readersInit, f)
}

// SetUpAndRun restores the database from the checkpoint, calls the init
// functions for the readers and writers, and then goes into a
// service loop processing the inputs from the readers.
func SetUpAndRun(conf *config.Config) error {
	// Read checkpoint file
	if len(*checkpoint) != 0 {
		readCheckpoint(*checkpoint, checkpointMap)
	}
	interval = time.Minute * time.Duration(*updateRate)
	lt, ok := checkpointMap[C_TIME]
	if !ok {
		lastInterval = time.Now().Truncate(interval)
	} else {
		var sec int64
		fmt.Sscanf(lt, "%d", &sec)
		lastInterval = time.Unix(sec, 0)
		if *Verbose {
			log.Printf("Last interval was %s\n", lastInterval.Format(time.UnixDate))
		}
	}
	input := make(chan Input, 200)
	for _, wi := range writersInit {
		if of, err := wi(conf); err != nil {
			return err
		} else if of != nil {
			outputs = append(outputs, of)
		}
	}
	for _, ri := range readersInit {
		if err := ri(conf, input); err != nil {
			return err
		}
	}
	tick := time.Tick(10 * time.Second)
	for {
		select {
		case r := <-input:
			checkInterval()
			h, ok := elements[r.Tag]
			if ok {
				h.Update(r.Value)
			} else {
				log.Printf("Unknown tag: %s\n", r.Tag)
			}
		case <-tick:
			checkInterval()
		}
	}
	return nil
}

// AddSumGauge adds a gauge that is part of a master gauge.
// If average is true, values are averaged, otherwise they are summed.
func AddSubGauge(base string, average bool) string {
	el, ok := elements[base]
	if !ok {
		el = NewMultiGauge(base, average)
		elements[base] = el
	}
	m := el.(*MultiGauge)
	tag := m.NextTag()
	g := NewGauge(checkpointMap[tag])
	m.Add(g)
	elements[tag] = g
	if *Verbose {
		log.Printf("Adding subgauge %s to %s\n", tag, base)
	}
	return tag
}

// AddSubAccum adds an accumulator that is part of a master accumulator.
func AddSubAccum(base string, resettable bool) string {
	el, ok := elements[base]
	if !ok {
		el = NewMultiAccum(base)
		elements[base] = el
	}
	m := el.(*MultiAccum)
	tag := m.NextTag()
	a := NewAccum(checkpointMap[tag], resettable)
	m.Add(a)
	elements[tag] = a
	if *Verbose {
		log.Printf("Adding subaccumulator %s to %s\n", tag, base)
	}
	return tag
}

// AddAverage adds an averaging element.
func AddAverage(name string) {
	elements[name] = NewAverage(checkpointMap[name])
	if *Verbose {
		log.Printf("Adding average %s\n", name)
	}
}

// AddGauge adds a new gauge to the database.
func AddGauge(name string) {
	elements[name] = NewGauge(checkpointMap[name])
	if *Verbose {
		log.Printf("Adding gauge %s\n", name)
	}
}

// AddAccum adds a new accumulator to the database.
func AddAccum(name string, resettable bool) {
	elements[name] = NewAccum(checkpointMap[name], resettable)
	if *Verbose {
		log.Printf("Adding accumulator %s\n", name)
	}
}

// checkInterval performs the interval update processing, calling the writers
// with the updated database. Some pre-write processing is done e.g if it is
// midnight, a flag is set.
// After write processing, the data is checkpointed, and the 'update' flag is
// cleared on all the elements.
func checkInterval() {
	// See if an update interval has passed.
	now := time.Now()
	if now.Sub(lastInterval) < interval {
		return
	}
	nowInterval := now.Truncate(interval)
	// Check for daily reset processing.
	newDay := nowInterval.YearDay() != lastInterval.YearDay()
	if *Verbose {
		log.Printf("Updating for interval %s\n", nowInterval.Format("15:04"))
		if newDay {
			log.Printf("New day reset")
		}
	}
	for tag, el := range elements {
		el.Interval(lastInterval, newDay)
		if *Verbose {
			log.Printf("Output: Tag: %5s, value: %f, updated: %v\n", tag, el.Get(), el.Updated())
		}
	}
	for _, wf := range outputs {
		wf(nowInterval)
	}
	if len(*checkpoint) != 0 {
		writeCheckpoint(*checkpoint, nowInterval)
	}
	for _, el := range elements {
		el.ClearUpdate()
	}
	lastInterval = nowInterval
}

// writerCheckpoint will save the current values of all the elements in the
// database to a file.
func writeCheckpoint(file string, now time.Time) {
	f, err := os.Create(file)
	if err != nil {
		log.Printf("Checkpoint file create: %s %v\n", file, err)
		return
	}
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	for n, e := range elements {
		s := e.Checkpoint()
		if len(s) != 0 {
			fmt.Fprintf(wr, "%s:%s\n", n, s)
		}
	}
	fmt.Fprintf(wr, "%s:%d\n", C_TIME, now.Unix())
}

// readCheckpoint restores the database elements from the last checkpoint.
func readCheckpoint(file string, cp map[string]string) {
	f, err := os.Open(file)
	if err != nil {
		log.Printf("Checkpoint file open: %s %v\n", file, err)
		return
	}
	defer f.Close()
	r := bufio.NewReader(f)
	lineno := 0
	for {
		lineno++
		s, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Checkpoint file read: %s line %d: %v\n", file, lineno, err)
			}
			return
		}
		s = strings.TrimSuffix(s, "\n")
		i := strings.IndexRune(s, ':')
		if i > 0 {
			cp[s[:i]] = s[i+1:]
		}
		if *Verbose {
			log.Printf("Checkpoint %s = %s\n", s[:i], s[i+1:])
		}
	}
}

// GetElement returns the database element named.
func GetElement(name string) Element {
	el, ok := elements[name]
	if !ok {
		return nil
	}
	return el
}
