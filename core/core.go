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
var checkpoint = flag.String("checkpoint", "/var/cache/MeterMan/checkpoint", "Checkpoint file")

type Input struct {
	Tag   string
	Value float64
}

// Element is the interface to each value that is being updated.
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

// Register a sender of telemetry data.
func RegisterWriter(f func(*config.Config) (func(time.Time), error)) {
	writersInit = append(writersInit, f)
}

// Register a receiver of telemetry data.
func RegisterReader(f func(*config.Config, chan<- Input) error) {
	readersInit = append(readersInit, f)
}

func SetUpAndRun(conf *config.Config) error {
	// Read checkpoint file
	if len(*checkpoint) != 0 {
		readCheckpoint(*checkpoint, checkpointMap)
	}
	interval = time.Minute * time.Duration(*updateRate)
	lastInterval = time.Now().Truncate(interval)
	input := make(chan Input, 200)
	for _, wi := range writersInit {
		if of, err := wi(conf); err != nil {
			return err
		} else {
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

func AddSubGauge(base string) string {
	el, ok := elements[base]
	if !ok {
		el = NewMultiGauge(base)
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

func AddSubAccum(base string) string {
	el, ok := elements[base]
	if !ok {
		el = NewMultiAccum(base)
		elements[base] = el
	}
	m := el.(*MultiAccum)
	tag := m.NextTag()
	a := NewAccum(checkpointMap[tag])
	m.Add(a)
	elements[tag] = a
	if *Verbose {
		log.Printf("Adding subaccumulator %s to %s\n", tag, base)
	}
	return tag
}

func AddAverage(name string) {
	elements[name] = NewAverage(checkpointMap[name])
	if *Verbose {
		log.Printf("Adding average %s\n", name)
	}
}

func AddGauge(name string) {
	elements[name] = NewGauge(checkpointMap[name])
	if *Verbose {
		log.Printf("Adding gauge %s\n", name)
	}
}

func AddAccum(name string) {
	elements[name] = NewAccum(checkpointMap[name])
	if *Verbose {
		log.Printf("Adding accumulator %s\n", name)
	}
}

func AddResettableAccum(name string) {
	a := NewAccum(checkpointMap[name])
	a.resettable = true
	elements[name] = a
	if *Verbose {
		log.Printf("Adding accumulator %s\n", name)
	}
}

func checkInterval() {
	// See if an update interval has passed.
	now := time.Now()
	if now.Sub(lastInterval) < interval {
		return
	}
	nowInterval := now.Truncate(interval)
	if *Verbose {
		log.Printf("Updating for interval %s\n", nowInterval.Format("15:04"))
	}
	// Check for daily reset processing.
	h, m, s := nowInterval.Clock()
	midnight := ((h + m + s) == 0)
	for tag, el := range elements {
		el.Interval(lastInterval, midnight)
		if *Verbose {
			log.Printf("Output: Tag: %5s, value: %f, updated: %v\n", tag, el.Get(), el.Updated())
		}
	}
	for _, wf := range outputs {
		wf(nowInterval)
	}
	if len(*checkpoint) != 0 {
		writeCheckpoint(*checkpoint)
	}
	for _, el := range elements {
		el.ClearUpdate()
	}
	lastInterval = nowInterval
}

func writeCheckpoint(file string) {
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
}

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

func GetElement(name string) Element {
	el, ok := elements[name]
	if !ok {
		return nil
	}
	return el
}
