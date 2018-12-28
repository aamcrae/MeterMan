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

const updateRate = 5

var dryrun = flag.Bool("dryrun", false, "Do not upload data")

var apikey string
var systemid string
var serverUrl string
var input chan Result = make(chan Result, 100)

var tpLast time.Time
var tpUpload time.Time
var tpAccum float64
var tpCurrent float64

var lastUpdate int

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
    go pvread(input, time.Tick(10 * time.Second))
    return input, nil
}

func pvread(in chan Result, tick <-chan time.Time) {
    for {
        select {
        case r := <-in:
            process(r.tag, r.value)
        case t := <-tick:
            // see if we have passed an update boundary.
            if (t.Minute() / updateRate) == lastUpdate {
                break
            }
            if *verbose {
                log.Printf("Update processing")
            }
            lastUpdate = t.Minute() / updateRate
            if tpLast.IsZero() {
                tpLast = t
                break
            }
            if !tpUpload.IsZero() {
                tpAccum += t.Sub(tpLast).Seconds() * tpCurrent
                avg := tpAccum / t.Sub(tpUpload).Seconds()
                upload(tpUpload, avg)
            }
            tpAccum = 0
            tpLast = t
            tpUpload = t
        }
    }
}

func process(tag string, value float64) {
    if *verbose {
        log.Printf("pvoutput: Tag: %s, value %f\n", tag, value)
    }
    now := time.Now()
    switch tag {
    case "IN":
    case "OUT":
    case "TP":
        if tpLast.IsZero() {
            tpCurrent = value
            tpLast = time.Now()
            break
        }
        tpCurrent = value
        tpAccum += now.Sub(tpLast).Seconds() * tpCurrent
        if *verbose {
            log.Printf("New accum = %f, current = %f, seconds = %d",
                tpAccum, tpCurrent, int(now.Sub(tpLast).Seconds()))
        }
        tpLast = now
    }
}

func upload(start time.Time, exportVal float64) error {
    if *verbose {
        log.Printf("Uploading %s: %f W", start.Format(time.RFC822), exportVal)
    }
    val := url.Values{}
    val.Add("d", start.Format("20060102"))
    val.Add("n", "1")
    val.Add("t", start.Format("15:04"))
    val.Add("v2", fmt.Sprintf("%d", int(exportVal * 1000)))
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
        log.Printf("Req failed: %v", err)
        return err
    }
    defer resp.Body.Close()
    if *verbose {
        log.Printf("Response is: %s", resp.Status)
    }
    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        err := fmt.Errorf("Error: %s: %s", resp.Status, body)
        log.Printf("%v", err)
        return err
    }
    return nil
}
