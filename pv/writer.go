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

var apikey string
var systemid string
var serverUrl string

func init() {
    core.RegisterWriter(pvoutputInit)
}

func pvoutputInit(conf *config.Config, data <-chan *core.Output) error {
    log.Printf("Registered pvoutput uploader as writer\n")
    if a, err := conf.GetArg("apikey"); err != nil {
        return err
    } else {
        apikey = a
    }
    if a, err := conf.GetArg("systemid"); err != nil {
        return err
    } else {
        systemid = a
    }
    if a, err := conf.GetArg("pvurl"); err != nil {
        return err
    } else {
        serverUrl = a
    }
    go writer(data)
    return nil
}

func writer(data <-chan *core.Output) {
    for {
        d := <-data
        el, ok := d.Values["TP"]
        if !ok {
            return
        }
        tp := el.(*core.Gauge)
        err := pvupload(d.Time, tp.Get())
        if err != nil {
            log.Printf("Update failed: %v", err)
        }
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
