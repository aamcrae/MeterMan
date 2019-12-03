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
// The package is configured as a section in the main config file
// under the '[csv]' section, and the parameters are:
//  [csv]
//  csv=<base directory>

package csv

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/aamcrae/MeterMan/db"
)

var csvUpdateRate = flag.Int("csvupdate", 5, "CSV update rate (in minutes)")

type writer struct {
	name string
	file *os.File
	buf  *bufio.Writer
}

const header = "#date,time"

var gauges []string = []string{"TP", "GEN-P", "VOLTS", "TEMP"}
var accums []string = []string{"IMP", "EXP", "GEN-T", "GEN-D", "IN", "OUT"}

type csv struct {
	d      *db.DB
	fpath  string
	day    int
	writer *writer
}

func init() {
	db.RegisterInit(csvInit)
}

// Returns a writer that writes daily CSV files in the form path/year/month/day
func csvInit(d *db.DB) error {
	var err error
	s := d.Config.GetSection("csv")
	if s == nil {
		return nil
	}
	p, err := s.GetArg("csv")
	if err != nil {
		return err
	}
	c := &csv{d: d, fpath: p}
	d.AddUpdate(c, time.Minute*time.Duration(*csvUpdateRate))
	log.Printf("Registered CSV as writer (updated every %d minutes)\n", *csvUpdateRate)
	return nil
}

func (c *csv) Update(last time.Time, now time.Time) {
	// Check for new day.
	if now.YearDay() != c.day {
		if c.writer != nil {
			c.writer.Close()
			c.writer = nil
		}
		var err error
		var created bool
		c.writer, created, err = NewWriter(c.fpath, now)
		if err != nil {
			log.Printf("%s: %v", c.fpath, err)
			return
		}
		if created {
			fmt.Fprint(c.writer, header)
			for _, s := range gauges {
				fmt.Fprintf(c.writer, ",%s", s)
			}
			for _, s := range accums {
				fmt.Fprintf(c.writer, ",%s,%s-DAILY", s, s)
			}
			fmt.Fprint(c.writer, "\n")
		}
		c.day = now.YearDay()
	}
	// Write values into file.
	if c.d.Trace {
		log.Printf("Writing CSV data to %s\n", c.writer.name)
	}
	fmt.Fprint(c.writer, now.Format("2006-01-02,15:04"))
	for _, s := range gauges {
		g := c.d.GetElement(s)
		fmt.Fprint(c.writer, ",")
		if g != nil && !g.Timestamp().Before(last) {
			fmt.Fprintf(c.writer, "%f", g.Get())
		}
	}
	for _, s := range accums {
		a := c.d.GetAccum(s)
		fmt.Fprint(c.writer, ",")
		if a != nil && !a.Timestamp().Before(last) {
			fmt.Fprintf(c.writer, "%f,%f", a.Get(), a.Daily())
		} else {
			fmt.Fprint(c.writer, ",")
		}
	}
	fmt.Fprint(c.writer, "\n")
	c.writer.Flush()
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
	return &writer{fn, f, bufio.NewWriter(f)}, created, nil
}

func (wr *writer) Write(p []byte) (n int, err error) {
	return wr.buf.Write(p)
}

func (wr *writer) Flush() error {
	return wr.buf.Flush()
}

func (wr *writer) Close() error {
	wr.buf.Flush()
	return wr.file.Close()
}
