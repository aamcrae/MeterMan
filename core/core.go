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

type Input struct {
    Tag string
    Value float64
}

type Output struct {
    Time time.Time
    Values map[string]Element
}

// Element is the interface to each value that is being updated.
type Element interface {
    Update(v float64)           // Update element with new value.
    PreWrite(t time.Time)       // Called to process the value before uploading.
    PostWrite()                 // Called after uploading.
    Updated() bool              // Return true if value has been updated in this interval.
    Get() float64               // Get the element's value
    Reset()                     // Daily reset.
    Checkpoint() string         // Return a checkpoint string.
}

var elements map[string]Element = map[string]Element{}

var interval time.Duration
var lastUpdate time.Time
var outputs []chan *Output

var writersInit []func (*config.Config, <-chan *Output) error
var readersInit []func (*config.Config, chan<- Input) error

// Register a sender of telemetry data.
func RegisterWriter(f func (*config.Config, <-chan *Output) error) {
    writersInit = append(writersInit, f)
}

// Register a receiver of telemetry data.
func RegisterReader(f func (*config.Config, chan<- Input) error) {
    readersInit = append(readersInit, f)
}

func SetUpAndRun(conf *config.Config) error {
    input := make(chan Input, 200)
    for _, wi := range writersInit {
        o := make(chan *Output, 100)
        outputs = append(outputs, o)
        if err := wi(conf, o); err != nil {
            return err
        }
    }
    for _, ri := range readersInit {
        if err := ri(conf, input); err != nil {
            return err
        }
    }
    cp := make(map[string]string)
    // Read checkpoint file
    if len(*checkpoint) != 0 {
        readCheckpoint(*checkpoint, cp)
    }
    interval = time.Minute * time.Duration(*updateRate)
    lastUpdate = time.Now().Truncate(interval)
    elements["TP"] = NewGauge(cp["TP"])
    elements["IN"] = NewAccum(cp["IN"])
    elements["OUT"] = NewAccum(cp["OUT"])
    tick := time.Tick(10 * time.Second)
    for {
        select {
        case r := <-input:
            checkInterval()
            if *Verbose {
                log.Printf("Tag: %5s value %f\n", r.Tag, r.Value)
            }
            h, ok := elements[r.Tag]
            if ok {
                h.Update(r.Value)
            }
        case <-tick:
            checkInterval()
        }
    }
    return nil
}

func checkInterval() {
    // See if an update interval has passed.
    now := time.Now()
    if now.Sub(lastUpdate) < interval {
        return
    }
    log.Printf("Updating now\n")
    lastUpdate = now.Truncate(interval)
    for n, el := range elements {
        el.PreWrite(lastUpdate)
        if *Verbose {
            var v float64
            switch e := el.(type) {
            case *Gauge:
                v = e.Get()
            case *Accum:
                v = e.Current()
            }
            log.Printf("Output: Tag: %5s, value %f\n", n, v)
        }
    }
    out := &Output{lastUpdate, elements}
    for _, wr := range outputs {
        wr <- out
    }
    for _, el := range elements {
        el.PostWrite()
    }
    if len(*checkpoint) != 0 {
        writeCheckpoint(*checkpoint)
    }
    // Check for daily reset processing.
    h, m, s := lastUpdate.Clock()
    if h + m + s == 0 {
        for _, el := range elements {
            el.Reset()
        }
    }
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
