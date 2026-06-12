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

// package csv writes the telemetered data to a daily CSV file.
// Under the base directory, year and month directories are
// created, and a daily file named as 'yyyy-mm-dd' is written.
// The package is configured as a section in the main YAML config file as:
//  csv: <base directory>

package csv

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aamcrae/MeterMan/core"
)

type CsvConfig struct {
	Base     string
	Interval int
}

type writer struct {
	io.WriteCloser
	name string
}

const header = "#date,time"

type field struct {
	name  string
	accum bool
}

var fields []field = []field{
	{"GEN-P", false},
	{"VOLTS", false},
	{"TEMP", false},
	{"IN-P", false},
	{"OUT-P", false},
	{"D-GEN-P", false},
	{"IMP", true},
	{"EXP", true},
	{"GEN-T", true},
	{"GEN-D", true},
	{"IN", true},
	{"OUT", true},
	{"FREQ", false},
	{"CHARGE-T", true},
	{"DISC-T", true},
	{"BATT-P", false},
	{"BATT-C", false},
}

const moduleName = "csv"

type csv struct {
	d      *core.DB
	fpath  string
	day    int
	writer *writer
	lines  int
	status atomic.Value
}

func init() {
	core.RegisterInit(csvInit)
}

// Returns a writer that writes daily CSV files in the form path/year/month/day
func csvInit(d *core.DB) error {
	var conf CsvConfig
	yaml, ok := d.Config[moduleName]
	if !ok {
		return nil
	}
	err := yaml.Decode(&conf)
	if err != nil {
		return err
	}
	interval := core.ConfigOrDefault(conf.Interval, 5) // Default of 5 minutes
	c := &csv{d: d, fpath: conf.Base}
	c.status.Store("init")
	if !d.Dryrun {
		d.AddExport(time.Minute*time.Duration(interval), 0, c.Run)
	}
	d.AddStatusPrinter(moduleName, c.Status)
	log.Printf("Registered CSV as writer, base directory %s, updating every %d minutes\n", conf.Base, interval)
	return nil
}

func (c *csv) Run(now time.Time) {
	// Generate the line to be written.
	var line strings.Builder
	fmt.Fprint(&line, now.Format("2006-01-02,15:04"))
	for _, f := range fields {
		e := c.d.GetElement(f.name)
		if e != nil && e.Fresh() {
			fmt.Fprintf(&line, ",%s", core.FmtFloat(e.Get()))
			// For accumulators, also store the daily accumulated value
			if f.accum {
				a := e.(core.Acc)
				fmt.Fprintf(&line, ",%s", core.FmtFloat(a.Daily()))
			}
		} else if f.accum {
			fmt.Fprint(&line, ",,")
		} else {
			fmt.Fprint(&line, ",")
		}
	}
	// Delegate the file I/O to a goroutine.
	go c.addCSV(now, line.String())
}

// addCSV writes the line to the CSV file, and if necessary
// creating a new file.
func (c *csv) addCSV(now time.Time, l string) {
	var b strings.Builder
	defer func() { c.status.Store(b.String()) }()
	fmt.Fprintf(&b, "%s: ", now.Format("2006-01-02 15:04"))
	// If a new day, close the current file and create a new CSV file
	if now.YearDay() != c.day {
		if c.writer != nil {
			c.writer.Close()
			c.lines = 0
		}
		var err error
		var created bool
		c.writer, created, err = NewWriter(c.fpath, now)
		if err != nil {
			log.Printf("%s: %v", c.fpath, err)
			fmt.Fprintf(&b, "NewWrite err: %v", err)
			return
		}
		if created {
			// Add CSV column header
			var h strings.Builder
			fmt.Fprint(&h, header)
			for _, f := range fields {
				fmt.Fprintf(&h, ",%s", f.name)
				if f.accum {
					fmt.Fprintf(&h, ",%s-DAILY", f.name)
				}
			}
			fmt.Fprintln(c.writer, h.String())
			c.lines++
		}
		c.day = now.YearDay()
	}
	// Write values into file.
	if c.d.Trace {
		log.Printf("Writing CSV line to %s\n", c.writer.name)
	}
	c.lines++
	fmt.Fprintln(c.writer, l)
	fmt.Fprintf(&b, "OK - file %s, lines %d", c.writer.name, c.lines)
}

func (c *csv) Status() string {
	return c.status.Load().(string)
}

// NewWriter creates a new file writer.
func NewWriter(p string, t time.Time) (*writer, bool, error) {
	// Create the path.
	dir := path.Join(p, t.Format("2006"), t.Format("01"))
	fn := path.Join(dir, t.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0775); err != nil {
		return nil, false, err
	}
	var created bool
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		// Create new file and write initial header.
		f, err = os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
		if err != nil {
			return nil, false, err
		}
		created = true
	}
	return &writer{f, fn}, created, nil
}
