// package csv writes the telemetered data to a daily CSV file.

package csv

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/aamcrae/MeterMan/core"
	"github.com/aamcrae/config"
)

type writer struct {
	name string
	file *os.File
	buf  *bufio.Writer
}

const header = "#date,time"

var gauges []string = []string{"TP", "GEN-P", "VOLTS", "TEMP"}
var accums []string = []string{"IMP", "EXP", "GEN-T", "GEN-D", "IN", "OUT"}
var filePath string
var currentDay int
var fileWriter *writer

func init() {
	core.RegisterWriter(csvInit)
}

// Writes daily CSV files in the form path/year/month/day
func csvInit(conf *config.Config) (func(time.Time), error) {
	log.Printf("Registered CSV as writer\n")
	var err error
	filePath, err = conf.GetSection("csv").GetArg("csv")
	if err != nil {
		return nil, err
	}
	return csvWriter, nil
}

func csvWriter(t time.Time) {
	if t.YearDay() != currentDay {
		if fileWriter != nil {
			fileWriter.Close()
			fileWriter = nil
		}
		var err error
		var created bool
		fileWriter, created, err = NewWriter(filePath, t)
		if err != nil {
			log.Printf("%v", err)
			return
		}
		if created {
			fmt.Fprint(fileWriter, header)
			for _, s := range gauges {
				fmt.Fprintf(fileWriter, ",%s", s)
			}
			for _, s := range accums {
				fmt.Fprintf(fileWriter, ",%s,%s-DAILY", s, s)
			}
			fmt.Fprint(fileWriter, "\n")
		}
		currentDay = t.YearDay()
	}
	// Write values into file.
	if *core.Verbose {
		log.Printf("Writing CSV data to %s\n", fileWriter.name)
	}
	fmt.Fprint(fileWriter, t.Format("2006-01-02,15:04"))
	for _, s := range gauges {
		g := core.GetElement(s)
		fmt.Fprint(fileWriter, ",")
		if g != nil && g.Updated() {
			fmt.Fprintf(fileWriter, "%f", g.Get())
		}
	}
	for _, s := range accums {
		a := core.GetAccum(s)
		fmt.Fprint(fileWriter, ",")
		if a != nil && a.Updated() {
			fmt.Fprintf(fileWriter, "%f,%f", a.Get(), a.Daily())
		} else {
			fmt.Fprint(fileWriter, ",")
		}
	}
	fmt.Fprint(fileWriter, "\n")
	fileWriter.Flush()
}

// NewWriter creates a new writer.
func NewWriter(p string, t time.Time) (*writer, bool, error) {
	// Create the path.
	dir := path.Join(p, t.Format("2006"), t.Format("01"))
	fn := path.Join(dir, t.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0775); err != nil {
		log.Printf("Mkdir %s: %v", dir, err)
	}
	var created bool
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		// Create new file and write initial header.
		f, err = os.OpenFile(fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
		if err != nil {
			log.Printf("Failed to create %s: %v", fn, err)
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
