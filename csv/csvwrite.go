package csv

import (
    "bufio"
    "fmt"
	"log"
    "os"
    "path"
    "time"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)

type Writer struct {
    name string
    file *os.File
    buf *bufio.Writer
}

const header = "#date,time"
var gauges []string = []string{"TP", "GEN-P", "VOLTS"}
var accums []string = []string{"IMP", "EXP", "GEN-T", "GEN-D", "IN", "OUT"}

func init() {
    core.RegisterWriter(csvInit)
}

// Writes daily CSV files in the form path/year/month/day
func csvInit(conf *config.Config, data <-chan *core.Output) error {
    log.Printf("Registered CSV as writer\n")
    p, err := conf.GetArg("csv")
    if err != nil {
        return err
    }
    go writer(p, data)
    return nil
}

func writer(path string, data <-chan *core.Output) {
    var day int
    var wr *Writer
    for {
        d := <-data
        if d.Time.YearDay() != day {
            if wr != nil {
                wr.Close()
                wr = nil
            }
            var err error
            var created bool
            wr, created, err = NewWriter(path, d.Time)
            if err != nil {
                log.Printf("%v", err)
                continue
            }
            if created {
                fmt.Fprint(wr, header)
                for _, s := range gauges {
                    fmt.Fprintf(wr, ",%s", s)
                }
                for _, s := range accums {
                    fmt.Fprintf(wr, ",%s,%s-DAILY", s, s)
                }
                fmt.Fprint(wr, "\n")
            }
            day = d.Time.YearDay()
        }
        // Write values into file.
        if *core.Verbose {
            log.Printf("Writing CSV data to %s\n", wr.name)
        }
        fmt.Fprint(wr, d.Time.Format("2006-01-02,15:04"))
        for _, s := range gauges {
            g := getGauge(d, s)
            fmt.Fprint(wr, ",")
            if g != nil {
                fmt.Fprintf(wr, "%f", g.Get())
            }
        }
        for _, s := range accums {
            a := getAcc(d, s)
            fmt.Fprint(wr, ",")
            if a != nil {
                fmt.Fprintf(wr, "%f,%f", a.Get(), a.Daily())
            } else {
                fmt.Fprint(wr, ",")
            }
        }
        fmt.Fprint(wr, "\n")
        wr.Flush()
    }
}

func NewWriter(p string, t time.Time) (*Writer, bool, error) {
    // Create the path.
    dir := path.Join(p, t.Format("2006"), t.Format("01"))
    fn := path.Join(dir, t.Format("2006-01-02"))
    if err := os.MkdirAll(dir, 0755); err != nil {
        log.Printf("Mkdir %s: %v", dir, err)
    }
    var created bool
    f, err := os.OpenFile(fn, os.O_APPEND | os.O_WRONLY, 0644)
    if err != nil {
        // Create new file and write initial header.
        f, err = os.OpenFile(fn, os.O_CREATE | os.O_APPEND | os.O_WRONLY, 0644)
        if err != nil {
            log.Printf("Failed to create %s: %v", fn, err)
            return nil, false, err
        }
        created = true
    }
    return &Writer{fn, f, bufio.NewWriter(f)}, created, nil
}

func (wr* Writer) Write(p []byte) (n int, err error) {
    return wr.buf.Write(p)
}

func (wr* Writer) Flush() (error) {
    return wr.buf.Flush()
}

func (wr* Writer) Close() (error) {
    wr.buf.Flush()
    return wr.file.Close()
}


func getGauge(d *core.Output, name string) (*core.Gauge) {
    el, ok := d.Values[name]
    if !ok || !el.Updated() {
        return nil
    }
    return el.(*core.Gauge)
}

func getAcc(d *core.Output, name string) (core.Acc) {
    el, ok := d.Values[name]
    if !ok || !el.Updated() {
        return nil
    }
    switch a := el.(type) {
    case *core.Accum:
        return a
    case  *core.MultiAccum:
        return a
    default:
        return nil
    }
}
