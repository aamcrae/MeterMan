package main

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
)

var dryrun = flag.Bool("dryrun", false, "Do not upload data")
var updateRate = flag.Int("update", 5, "Update rate (in minutes)")

var apikey string
var systemid string
var serverUrl string
var input chan Result = make(chan Result, 100)

var interval time.Duration
var tpLast time.Time
var lastUpdate time.Time
var tpAccum float64
var tpCurrent float64

var exportCurrent float64
var importCurrent float64

func init() {
    WritersInit = append(WritersInit, pvoutputInit)
}

func pvoutputInit(conf *config.Config) (chan<- Result, error) {
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
    tpLast = lastUpdate
    go pvread(input, time.Tick(10 * time.Second))
    return input, nil
}

func pvread(in chan Result, tick <-chan time.Time) {
    for {
        select {
        case r := <-in:
            process(r.tag, r.value)
        case t := <-tick:
            // See if an update interval has passed.
            if t.Sub(lastUpdate) < interval {
                break
            }
            // Round out the accumulated power value to the interval boundary.
            tpAccum += t.Sub(tpLast).Seconds() * tpCurrent
            err := pvupload(lastUpdate, tpAccum / t.Sub(lastUpdate).Seconds())
            if err != nil {
                log.Printf("Update failed: %v", err)
            }
            tpAccum = 0
            lastUpdate = t.Truncate(interval)
            tpLast = t
        }
    }
}

func process(tag string, value float64) {
    switch tag {
    case "IN":
        importCurrent = value
    case "OUT":
        exportCurrent = value
    case "TP":
        now := time.Now()
        tpCurrent = value
        tpAccum += now.Sub(tpLast).Seconds() * tpCurrent
        if *verbose {
            log.Printf("New power accum = %f, current = %f, seconds = %d",
                tpAccum, tpCurrent, int(now.Sub(tpLast).Seconds()))
        }
        tpLast = now
    }
}

func pvupload(start time.Time, exportVal float64) error {
    if *verbose {
        log.Printf("Uploading power %s: %f W", start.Format(time.RFC822), exportVal)
    }
    val := url.Values{}
    val.Add("d", start.Format("20060102"))
    val.Add("n", "1")
    val.Add("t", start.Format("15:04"))
    val.Add("v4", fmt.Sprintf("%d", int(exportVal * 1000)))
    req, err := http.NewRequest("POST", serverUrl, strings.NewReader(val.Encode()))
    if err != nil {
        log.Printf("NewRequest failed: %v", err)
        return err
    }
    req.Header.Add("X-Pvoutput-Apikey", apikey)
    req.Header.Add("X-Pvoutput-SystemId", systemid)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    if *verbose || *dryrun {
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
    if *verbose {
        log.Printf("Response is: %s", resp.Status)
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        err := fmt.Errorf("Error: %s: %s", resp.Status, body)
        return err
    }
    return nil
}
