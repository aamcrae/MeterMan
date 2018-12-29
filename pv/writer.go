package pv

import (
    "flag"
    "fmt"
    "io/ioutil"
	"log"
	"net/http"
    "net/url"
    "strings"
    "time"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)

var dryrun = flag.Bool("dryrun", false, "Do not upload data")
var updateRate = flag.Int("update", 5, "Update rate (in minutes)")

type Gauge struct {
    acc float64
    current float64
    last time.Time
    updated bool
}

type Accum struct {
    value float64
    initial float64
    midnight float64
}

var apikey string
var systemid string
var serverUrl string
var input chan core.Result = make(chan core.Result, 100)

var interval time.Duration
var lastUpdate time.Time

var tp *Gauge
var exportEnergy *Accum
var importEnergy *Accum

func init() {
    core.RegisterWriter(pvoutputInit)
}

func pvoutputInit(conf *config.Config) (chan<- core.Result, error) {
    if a, err := conf.GetArg("apikey"); err != nil {
        return nil, err
    } else {
        apikey = a
    }
    if a, err := conf.GetArg("systemid"); err != nil {
        return nil, err
    } else {
        systemid = a
    }
    if a, err := conf.GetArg("pvurl"); err != nil {
        return nil, err
    } else {
        serverUrl = a
    }
    interval = time.Minute * time.Duration(*updateRate)
    lastUpdate = time.Now().Truncate(interval)
    tp = &Gauge{0, 0, lastUpdate, false}
    importEnergy = new(Accum)
    exportEnergy = new(Accum)
    go pvread(input, time.Tick(10 * time.Second))
    return input, nil
}

func pvread(in chan core.Result, tick <-chan time.Time) {
    for {
        select {
        case r := <-in:
            checkInterval()
            process(r.Tag, r.Value)
        case <-tick:
            checkInterval()
        }
    }
}

func checkInterval() {
    // See if an update interval has passed.
    now := time.Now()
    if now.Sub(lastUpdate) < interval {
        return
    }
    lastUpdate = now.Truncate(interval)
    avg := tp.get(lastUpdate)
    err := pvupload(lastUpdate, avg)
    if err != nil {
        log.Printf("Update failed: %v", err)
    }
    // Check for midnight processing.
    h, m, s := lastUpdate.Clock()
    if h + m + s == 0 {
        exportEnergy.reset()
        importEnergy.reset()
    }
}

func process(tag string, value float64) {
    switch tag {
    case "IN":
        importEnergy.update(value)
        if *core.Verbose {
            log.Printf("Import daily: %f, total %f\n", importEnergy.daily(), importEnergy.total())
        }
    case "OUT":
        exportEnergy.update(value)
        if *core.Verbose {
            log.Printf("Export daily: %f, total %f\n", exportEnergy.daily(), exportEnergy.total())
        }
    case "TP":
        tp.update(value)
    }
}

func pvupload(t time.Time, exportVal float64) error {
    if *core.Verbose {
        log.Printf("Uploading power %s: %f W", t.Format(time.RFC822), exportVal)
    }
    val := url.Values{}
    val.Add("d", t.Format("20060102"))
    val.Add("n", "1")
    val.Add("t", t.Format("15:04"))
    if exportVal >= 0 {
        val.Add("v4", fmt.Sprintf("%d", int(exportVal * 1000)))
    } else {
        val.Add("v2", fmt.Sprintf("%d", int(exportVal * -1000)))
    }
    req, err := http.NewRequest("POST", serverUrl, strings.NewReader(val.Encode()))
    if err != nil {
        log.Printf("NewRequest failed: %v", err)
        return err
    }
    req.Header.Add("X-Pvoutput-Apikey", apikey)
    req.Header.Add("X-Pvoutput-SystemId", systemid)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    if *core.Verbose || *dryrun {
        log.Printf("req: %s (size %d)", val.Encode(), req.ContentLength)
        if *dryrun {
            return nil
        }
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if *core.Verbose {
        log.Printf("Response is: %s", resp.Status)
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        err := fmt.Errorf("Error: %s: %s", resp.Status, body)
        return err
    }
    return nil
}

func (g *Gauge) update(value float64) {
    now := time.Now()
    g.current = value
    g.acc += now.Sub(g.last).Seconds() * g.current
    g.last = now
    g.updated = true
}

func (g *Gauge) get(t time.Time) float64 {
    g.acc += t.Sub(g.last).Seconds() * g.current
    avg := g.acc / interval.Seconds()
    g.acc = 0
    g.last = t
    g.current = 0
    g.updated = false
    return avg
}

func (a *Accum) update(v float64) {
    if a.value == 0 {
        a.initial = v
        a.midnight = v
    }
    a.value = v
}

func (a *Accum) reset() {
    a.midnight = a.value
}

func (a *Accum) total() float64 {
    return a.value - a.initial
}

func (a *Accum) daily() float64 {
    return a.value - a.midnight
}
