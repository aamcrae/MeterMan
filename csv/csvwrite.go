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
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aamcrae/MeterMan/db"
	"github.com/aamcrae/MeterMan/lib"
)

type CsvConfig struct {
	Base     string
	Interval int
}

type writer struct {
	name string
	file *os.File
	buf  *bufio.Writer
}

const defaultInterval = 5

const header = "#date,time"

var elements []string = []string{"GEN-P", "VOLTS", "TEMP", "IN-P", "OUT-P", "D-GEN-P"}
var accums []string = []string{"IMP", "EXP", "GEN-T", "GEN-D", "IN", "OUT"}

const moduleName = "csv"

type csv struct {
	d      *db.DB
	fpath  string
	day    int
	writer *writer
	status string
}

func init() {
	db.RegisterInit(csvInit)
}

// Returns a writer that writes daily CSV files in the form path/year/month/day
func csvInit(d *db.DB) error {
	var conf CsvConfig
	yaml, ok := d.Config[moduleName]
	if !ok {
		return nil
	}
	err := yaml.Decode(&conf)
	if err != nil {
		return err
	}
	interval := lib.ConfigOrDefault(conf.Interval, defaultInterval)
	c := &csv{d: d, fpath: conf.Base, status: "init"}
	if !d.Dryrun {
		d.AddCallback(time.Minute*time.Duration(interval), c.Run)
	}
	d.AddStatusPrinter(moduleName, c.Status)
	log.Printf("Registered CSV as writer, base directory %s, updating every %d minutes\n", conf.Base, interval)
	return nil
}

func (c *csv) Run(now time.Time) {
	var b strings.Builder
	defer func() {c.status = b.String()}()
	fmt.Fprintf(&b, "%s: ", now.Format("2006-01-02 15:04"))
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
			fmt.Fprintf(&b, "NewWriter Err: - %v", err)
			return
		}
		if created {
			fmt.Fprint(c.writer, header)
			for _, s := range elements {
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
	for _, s := range elements {
		g := c.d.GetElement(s)
		fmt.Fprint(c.writer, ",")
		if g != nil && g.Fresh() {
			fmt.Fprintf(c.writer, "%s", lib.FmtFloat(g.Get()))
		}
	}
	for _, s := range accums {
		a := c.d.GetAccum(s)
		fmt.Fprint(c.writer, ",")
		if a != nil && a.Fresh() {
			fmt.Fprintf(c.writer, "%s,%s", lib.FmtFloat(a.Get()), lib.FmtFloat(a.Daily()))
		} else {
			fmt.Fprint(c.writer, ",")
		}
	}
	fmt.Fprint(c.writer, "\n")
	fmt.Fprintf(&b, "OK - file %s", c.writer.name)
	c.writer.Flush()
}

func (c *csv) Status() string {
	return c.status
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
